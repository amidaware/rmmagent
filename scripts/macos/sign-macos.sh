#!/bin/bash

# Code signing script for rmmagent macOS binaries
# Signs all binaries (amd64, arm64, universal) and app bundles

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the root directory of the project
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/Output/macos"

# Check if certificate is set
if [ -z "$MACOS_SIGN_CERT" ]; then
    echo -e "${RED}Error: MACOS_SIGN_CERT environment variable not set${NC}"
    echo "Please set it to your Developer ID Application certificate name:"
    echo "  export MACOS_SIGN_CERT=\"Developer ID Application: Your Name (TEAMID)\""
    echo ""
    echo "To find your certificate, run:"
    echo "  security find-identity -v -p codesigning"
    exit 1
fi

echo -e "${BLUE}===========================================================${NC}"
echo -e "${BLUE}Code Signing rmmagent Binaries${NC}"
echo -e "${BLUE}===========================================================${NC}"
echo "Certificate: $MACOS_SIGN_CERT"
echo "Build directory: $BUILD_DIR"
echo ""

# Check if build directory exists
if [ ! -d "$BUILD_DIR" ]; then
    echo -e "${RED}Error: Build directory not found: $BUILD_DIR${NC}"
    echo "Please run the build script first."
    exit 1
fi

# Function to sign a binary
sign_binary() {
    local binary="$1"
    local binary_name=$(basename "$binary")
    local arch=$(echo "$binary" | sed 's|.*/build/Output/macos/\([^/]*\)/.*|\1|')

    echo -e "${YELLOW}Signing: ${binary_name} (${arch})${NC}"

    codesign --sign "$MACOS_SIGN_CERT" \
             --timestamp \
             --options runtime \
             --force \
             "$binary"

    # Verify the signature
    if codesign -vvv --deep --strict "$binary" 2>&1 | grep -q "valid on disk"; then
        echo -e "${GREEN}✓ Successfully signed and verified: ${binary_name} (${arch})${NC}"
    else
        echo -e "${RED}✗ Signature verification failed: ${binary_name} (${arch})${NC}"
        exit 1
    fi
}

# Step 1: Sign all standalone binaries
echo -e "${BLUE}[Step 1/2] Signing standalone binaries${NC}"
echo ""

# Find all rmmagent binaries (excluding those inside .app bundles)
BINARIES=()
while IFS= read -r -d $'\0' binary; do
    BINARIES+=("$binary")
done < <(find "$BUILD_DIR" -type f -name "rmmagent" ! -path "*.app/*" -print0)

if [ ${#BINARIES[@]} -eq 0 ]; then
    echo -e "${YELLOW}No binaries found to sign${NC}"
else
    for binary in "${BINARIES[@]}"; do
        sign_binary "$binary"
    done

    # Update app bundle binaries if they exist
    for binary in "${BINARIES[@]}"; do
        # Check if this is the universal binary
        if [[ "$binary" == */universal/rmmagent ]]; then
            APP_BUNDLE="$(dirname "$binary")/rmmagent.app"
            if [ -d "$APP_BUNDLE" ]; then
                echo ""
                echo -e "${YELLOW}Updating app bundle with signed universal binary${NC}"
                cp "$binary" "$APP_BUNDLE/Contents/MacOS/rmmagent"
                chmod +x "$APP_BUNDLE/Contents/MacOS/rmmagent"
                echo -e "${GREEN}✓ App bundle updated${NC}"
            fi
        fi
    done
fi

echo ""

# Step 2: Sign app bundles
echo -e "${BLUE}[Step 2/2] Signing app bundles${NC}"
echo ""

APP_BUNDLES=()
while IFS= read -r -d $'\0' bundle; do
    APP_BUNDLES+=("$bundle")
done < <(find "$BUILD_DIR" -type d -name "*.app" -print0)

if [ ${#APP_BUNDLES[@]} -eq 0 ]; then
    echo -e "${YELLOW}No app bundles found${NC}"
else
    for bundle in "${APP_BUNDLES[@]}"; do
        bundle_name=$(basename "$bundle")
        echo -e "${YELLOW}Signing app bundle: ${bundle_name}${NC}"

        codesign --sign "$MACOS_SIGN_CERT" \
                 --timestamp \
                 --options runtime \
                 --deep \
                 --force \
                 "$bundle"

        # Verify the signature
        if codesign -vvv --deep --strict "$bundle" 2>&1 | grep -q "valid on disk"; then
            echo -e "${GREEN}✓ Successfully signed and verified: ${bundle_name}${NC}"
        else
            echo -e "${RED}✗ Signature verification failed: ${bundle_name}${NC}"
            exit 1
        fi
    done
fi

echo ""
echo -e "${GREEN}===========================================================${NC}"
echo -e "${GREEN}Code signing completed successfully!${NC}"
echo -e "${GREEN}===========================================================${NC}"
