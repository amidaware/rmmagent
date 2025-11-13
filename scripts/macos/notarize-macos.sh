#!/bin/bash

# Notarization script for rmmagent macOS binaries and app bundles
# Submits signed binaries to Apple's notarization service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
KEYCHAIN_PROFILE="rmmagent-notary"

# Get the root directory of the project
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/Output/macos"

# Parse command-line arguments
PARALLEL=false
VERBOSE=false

show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Notarizes all rmmagent binaries and app bundles in the build directory.

Options:
    --parallel      Submit all items for notarization concurrently
    --verbose       Show detailed notarytool output
    --help          Show this help message

Requirements:
    - Binaries must be signed before notarization
    - Keychain profile "$KEYCHAIN_PROFILE" must be configured

To setup the keychain profile (one-time):
    xcrun notarytool store-credentials "$KEYCHAIN_PROFILE" \\
        --apple-id "your@email.com" \\
        --team-id "YOUR_TEAM_ID" \\
        --password "xxxx-xxxx-xxxx-xxxx"

    Note: Password should be an app-specific password from:
    https://appleid.apple.com → Security → App-Specific Passwords

EOF
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --parallel)
            PARALLEL=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

echo -e "${BLUE}===========================================================${NC}"
echo -e "${BLUE}Notarizing rmmagent Binaries${NC}"
echo -e "${BLUE}===========================================================${NC}"
echo "Build directory: $BUILD_DIR"
echo "Keychain profile: $KEYCHAIN_PROFILE"
echo "Parallel mode: $PARALLEL"
echo "Verbose mode: $VERBOSE"
echo ""

# Check if build directory exists
if [ ! -d "$BUILD_DIR" ]; then
    echo -e "${RED}Error: Build directory not found: $BUILD_DIR${NC}"
    echo "Please run the build script first."
    exit 1
fi

# Check if keychain profile exists
if ! xcrun notarytool history --keychain-profile "$KEYCHAIN_PROFILE" &>/dev/null; then
    echo -e "${RED}Error: Keychain profile '$KEYCHAIN_PROFILE' not found${NC}"
    echo ""
    echo "Please set up the keychain profile first:"
    echo "  xcrun notarytool store-credentials \"$KEYCHAIN_PROFILE\" \\"
    echo "      --apple-id \"your@email.com\" \\"
    echo "      --team-id \"YOUR_TEAM_ID\" \\"
    echo "      --password \"xxxx-xxxx-xxxx-xxxx\""
    echo ""
    echo "Note: Password should be an app-specific password from:"
    echo "  https://appleid.apple.com → Security → App-Specific Passwords"
    exit 1
fi

# Create temporary directory for ZIP files
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Function to notarize a standalone binary
notarize_binary() {
    local binary="$1"
    local binary_name=$(basename "$binary")
    local binary_dir=$(dirname "$binary")
    local binary_file=$(basename "$binary")
    local arch=$(echo "$binary" | sed 's|.*/build/Output/macos/\([^/]*\)/.*|\1|')
    local zip_path="$TEMP_DIR/${binary_name}-${arch}.zip"

    echo -e "${YELLOW}Notarizing: ${binary_name} (${arch})${NC}"

    # Create ZIP archive (without --keepParent for standalone binaries)
    (cd "$binary_dir" && ditto -c -k "$binary_file" "$zip_path")

    # Submit for notarization
    local submit_output
    if [ "$VERBOSE" = true ]; then
        submit_output=$(xcrun notarytool submit "$zip_path" \
            --keychain-profile "$KEYCHAIN_PROFILE" \
            --wait \
            --timeout 30m 2>&1)
        echo "$submit_output"
    else
        submit_output=$(xcrun notarytool submit "$zip_path" \
            --keychain-profile "$KEYCHAIN_PROFILE" \
            --wait \
            --timeout 30m 2>&1)
    fi

    if echo "$submit_output" | grep -q "status: Accepted"; then
        echo -e "${GREEN}✓ Successfully notarized: ${binary_name} (${arch})${NC}"
        return 0
    else
        echo -e "${RED}✗ Notarization failed: ${binary_name} (${arch})${NC}"
        if [ "$VERBOSE" = false ]; then
            echo "$submit_output"
        fi
        return 1
    fi
}

# Function to notarize an app bundle
notarize_bundle() {
    local bundle="$1"
    local bundle_name=$(basename "$bundle")
    local bundle_dir=$(dirname "$bundle")
    local arch=$(echo "$bundle" | sed 's|.*/build/Output/macos/\([^/]*\)/.*|\1|')
    local zip_path="$TEMP_DIR/${bundle_name%.*}-${arch}.zip"

    echo -e "${YELLOW}Notarizing: ${bundle_name} (${arch})${NC}"

    # Create ZIP archive (with --keepParent for app bundles)
    ditto -c -k --keepParent "$bundle" "$zip_path"

    # Submit for notarization
    local submit_output
    if [ "$VERBOSE" = true ]; then
        submit_output=$(xcrun notarytool submit "$zip_path" \
            --keychain-profile "$KEYCHAIN_PROFILE" \
            --wait \
            --timeout 30m 2>&1)
        echo "$submit_output"
    else
        submit_output=$(xcrun notarytool submit "$zip_path" \
            --keychain-profile "$KEYCHAIN_PROFILE" \
            --wait \
            --timeout 30m 2>&1)
    fi

    if echo "$submit_output" | grep -q "status: Accepted"; then
        echo -e "${GREEN}✓ Successfully notarized: ${bundle_name} (${arch})${NC}"
        return 0
    else
        echo -e "${RED}✗ Notarization failed: ${bundle_name} (${arch})${NC}"
        if [ "$VERBOSE" = false ]; then
            echo "$submit_output"
        fi
        return 1
    fi
}

# Find all release binaries (exclude DEBUG builds)
BINARIES=()
while IFS= read -r -d $'\0' binary; do
    # Exclude binaries inside app bundles
    if [[ ! "$binary" =~ \.app/ ]]; then
        BINARIES+=("$binary")
    fi
done < <(find "$BUILD_DIR" -type f -name "rmmagent" -print0)

# Find all app bundles
APP_BUNDLES=()
while IFS= read -r -d $'\0' bundle; do
    APP_BUNDLES+=("$bundle")
done < <(find "$BUILD_DIR" -type d -name "*.app" -print0)

# Calculate total items
TOTAL_ITEMS=$((${#BINARIES[@]} + ${#APP_BUNDLES[@]}))

if [ $TOTAL_ITEMS -eq 0 ]; then
    echo -e "${YELLOW}No items found to notarize${NC}"
    exit 0
fi

echo -e "${BLUE}Found ${TOTAL_ITEMS} item(s) to notarize:${NC}"
echo "  - ${#BINARIES[@]} standalone binaries"
echo "  - ${#APP_BUNDLES[@]} app bundles"
echo ""

# Counters
SUCCESS_COUNT=0
FAILURE_COUNT=0

# Process binaries
if [ ${#BINARIES[@]} -gt 0 ]; then
    echo -e "${BLUE}Notarizing standalone binaries${NC}"
    echo ""

    if [ "$PARALLEL" = true ]; then
        # Parallel mode: submit all concurrently
        PIDS=()
        for binary in "${BINARIES[@]}"; do
            notarize_binary "$binary" &
            PIDS+=($!)
        done

        # Wait for all to complete
        for pid in "${PIDS[@]}"; do
            if wait "$pid"; then
                ((SUCCESS_COUNT++))
            else
                ((FAILURE_COUNT++))
            fi
        done
    else
        # Sequential mode: one at a time
        for binary in "${BINARIES[@]}"; do
            if notarize_binary "$binary"; then
                ((SUCCESS_COUNT++))
            else
                ((FAILURE_COUNT++))
            fi
            echo ""
        done
    fi
fi

# Process app bundles
if [ ${#APP_BUNDLES[@]} -gt 0 ]; then
    echo -e "${BLUE}Notarizing app bundles${NC}"
    echo ""

    if [ "$PARALLEL" = true ]; then
        # Parallel mode: submit all concurrently
        PIDS=()
        for bundle in "${APP_BUNDLES[@]}"; do
            notarize_bundle "$bundle" &
            PIDS+=($!)
        done

        # Wait for all to complete
        for pid in "${PIDS[@]}"; do
            if wait "$pid"; then
                ((SUCCESS_COUNT++))
            else
                ((FAILURE_COUNT++))
            fi
        done
    else
        # Sequential mode: one at a time
        for bundle in "${APP_BUNDLES[@]}"; do
            if notarize_bundle "$bundle"; then
                ((SUCCESS_COUNT++))
            else
                ((FAILURE_COUNT++))
            fi
            echo ""
        done
    fi
fi

# Summary
echo ""
echo -e "${BLUE}===========================================================${NC}"
echo -e "${BLUE}Notarization Summary${NC}"
echo -e "${BLUE}===========================================================${NC}"
echo "Total items: $TOTAL_ITEMS"
echo -e "${GREEN}Successful: $SUCCESS_COUNT${NC}"
if [ $FAILURE_COUNT -gt 0 ]; then
    echo -e "${RED}Failed: $FAILURE_COUNT${NC}"
else
    echo "Failed: $FAILURE_COUNT"
fi
echo ""

if [ $FAILURE_COUNT -eq 0 ]; then
    echo -e "${GREEN}All items notarized successfully!${NC}"
    exit 0
else
    echo -e "${RED}Some items failed notarization${NC}"
    exit 1
fi
