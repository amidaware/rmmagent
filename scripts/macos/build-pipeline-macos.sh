#!/bin/bash

# macOS build pipeline orchestrator for rmmagent
# Coordinates code signing, notarization, and stapling

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

# Configuration (can be overridden by environment variables)
DO_SIGN="${DO_SIGN:-true}"
DO_NOTARIZE="${DO_NOTARIZE:-false}"
DO_STAPLE="${DO_STAPLE:-false}"

echo -e "${GREEN}===========================================================${NC}"
echo -e "${GREEN}rmmagent macOS Build Pipeline${NC}"
echo -e "${GREEN}===========================================================${NC}"
echo ""
echo "Configuration:"
echo "  Code Signing: $DO_SIGN"
echo "  Notarization: $DO_NOTARIZE"
echo "  Stapling: $DO_STAPLE"
echo ""

# Validate configuration
if [ "$DO_SIGN" = "true" ]; then
    if [ -z "$MACOS_SIGN_CERT" ]; then
        echo -e "${RED}Error: DO_SIGN=true but MACOS_SIGN_CERT is not set${NC}"
        echo ""
        echo "Please set your Developer ID Application certificate:"
        echo "  export MACOS_SIGN_CERT=\"Developer ID Application: Your Name (TEAMID)\""
        echo ""
        echo "To find your certificate, run:"
        echo "  security find-identity -v -p codesigning"
        exit 1
    fi
fi

if [ "$DO_NOTARIZE" = "true" ]; then
    if ! xcrun notarytool history --keychain-profile "rmmagent-notary" &>/dev/null; then
        echo -e "${RED}Error: DO_NOTARIZE=true but keychain profile 'rmmagent-notary' not found${NC}"
        echo ""
        echo "Please set up the keychain profile first:"
        echo "  xcrun notarytool store-credentials \"rmmagent-notary\" \\"
        echo "      --apple-id \"your@email.com\" \\"
        echo "      --team-id \"YOUR_TEAM_ID\" \\"
        echo "      --password \"xxxx-xxxx-xxxx-xxxx\""
        echo ""
        echo "Note: Password should be an app-specific password from:"
        echo "  https://appleid.apple.com → Security → App-Specific Passwords"
        exit 1
    fi
fi

# Check if build directory exists
if [ ! -d "$BUILD_DIR" ]; then
    echo -e "${RED}Error: Build directory not found: $BUILD_DIR${NC}"
    echo "Please run the build script first."
    exit 1
fi

STEP_NUM=0
TOTAL_STEPS=0

# Calculate total steps
[ "$DO_SIGN" = "true" ] && ((TOTAL_STEPS++))
[ "$DO_NOTARIZE" = "true" ] && ((TOTAL_STEPS++))
[ "$DO_STAPLE" = "true" ] && ((TOTAL_STEPS++))

if [ $TOTAL_STEPS -eq 0 ]; then
    echo -e "${YELLOW}No operations to perform (all disabled)${NC}"
    exit 0
fi

# Step 1: Code Signing
if [ "$DO_SIGN" = "true" ]; then
    ((STEP_NUM++))
    echo ""
    echo -e "${BLUE}===========================================================${NC}"
    echo -e "${BLUE}[$STEP_NUM/$TOTAL_STEPS] Code Signing${NC}"
    echo -e "${BLUE}===========================================================${NC}"
    echo ""

    "$SCRIPT_DIR/sign-macos.sh"
fi

# Step 2: Notarization
if [ "$DO_NOTARIZE" = "true" ]; then
    ((STEP_NUM++))
    echo ""
    echo -e "${BLUE}===========================================================${NC}"
    echo -e "${BLUE}[$STEP_NUM/$TOTAL_STEPS] Notarization${NC}"
    echo -e "${BLUE}===========================================================${NC}"
    echo ""

    "$SCRIPT_DIR/notarize-macos.sh"
fi

# Step 3: Stapling
if [ "$DO_STAPLE" = "true" ]; then
    ((STEP_NUM++))
    echo ""
    echo -e "${BLUE}===========================================================${NC}"
    echo -e "${BLUE}[$STEP_NUM/$TOTAL_STEPS] Stapling${NC}"
    echo -e "${BLUE}===========================================================${NC}"
    echo ""

    # Find all app bundles
    APP_BUNDLES=()
    while IFS= read -r -d $'\0' bundle; do
        APP_BUNDLES+=("$bundle")
    done < <(find "$BUILD_DIR" -type d -name "*.app" -print0)

    if [ ${#APP_BUNDLES[@]} -eq 0 ]; then
        echo -e "${YELLOW}No app bundles found to staple${NC}"
    else
        for bundle in "${APP_BUNDLES[@]}"; do
            bundle_name=$(basename "$bundle")
            echo -e "${YELLOW}Stapling: ${bundle_name}${NC}"

            if xcrun stapler staple "$bundle" 2>&1; then
                echo -e "${GREEN}✓ Successfully stapled: ${bundle_name}${NC}"

                # Validate stapling
                if xcrun stapler validate "$bundle" >/dev/null 2>&1; then
                    echo -e "${GREEN}✓ Stapling validated: ${bundle_name}${NC}"
                fi
            else
                # Stapling can fail for standalone binaries (Error 73), which is expected
                echo -e "${YELLOW}Note: Stapling failed (this is normal for standalone binaries)${NC}"
            fi
            echo ""
        done
    fi

    # Note about standalone binaries
    echo -e "${BLUE}Note:${NC} Standalone binaries cannot be stapled (Error 73 is expected)."
    echo "They are still notarized and will verify online on first run."
fi

# Summary
echo ""
echo -e "${GREEN}===========================================================${NC}"
echo -e "${GREEN}Pipeline Summary${NC}"
echo -e "${GREEN}===========================================================${NC}"
echo ""
echo "Completed operations:"
[ "$DO_SIGN" = "true" ] && echo -e "${GREEN}  ✓ Code Signing${NC}" || echo "  - Code Signing (skipped)"
[ "$DO_NOTARIZE" = "true" ] && echo -e "${GREEN}  ✓ Notarization${NC}" || echo "  - Notarization (skipped)"
[ "$DO_STAPLE" = "true" ] && echo -e "${GREEN}  ✓ Stapling${NC}" || echo "  - Stapling (skipped)"
echo ""
echo -e "${GREEN}Build pipeline completed successfully!${NC}"
