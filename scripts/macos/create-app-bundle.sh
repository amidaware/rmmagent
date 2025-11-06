#!/bin/bash

# Creates a macOS app bundle from rmmagent binary
# Usage: ./create-app-bundle.sh [path-to-rmmagent-binary]
# Default: build/Output/macos/universal/rmmagent

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

# Default binary path
DEFAULT_BINARY="$PROJECT_ROOT/build/Output/macos/universal/rmmagent"
BINARY_PATH="${1:-$DEFAULT_BINARY}"

echo -e "${BLUE}===========================================================${NC}"
echo -e "${BLUE}Creating macOS App Bundle for rmmagent${NC}"
echo -e "${BLUE}===========================================================${NC}"
echo ""

# Check if binary exists
if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}Error: Binary not found: $BINARY_PATH${NC}"
    echo ""
    echo "Usage: $(basename "$0") [path-to-rmmagent-binary]"
    echo "Default: build/Output/macos/universal/rmmagent"
    exit 1
fi

# Verify it's a Mach-O executable
if ! file "$BINARY_PATH" | grep -q "Mach-O"; then
    echo -e "${RED}Error: Not a valid Mach-O executable: $BINARY_PATH${NC}"
    exit 1
fi

echo "Binary: $BINARY_PATH"

# Extract version from main.go
VERSION_FILE="$PROJECT_ROOT/main.go"
if [ ! -f "$VERSION_FILE" ]; then
    echo -e "${RED}Error: main.go not found at: $VERSION_FILE${NC}"
    exit 1
fi

# Extract version using grep/sed
VERSION=$(grep -E '^\s*version\s*=\s*"' "$VERSION_FILE" | sed -E 's/.*version\s*=\s*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo -e "${YELLOW}Warning: Could not extract version from main.go, using default${NC}"
    VERSION="1.0.0"
fi

echo "Version: $VERSION"
echo ""

# Determine app bundle path (same directory as binary)
BINARY_DIR=$(dirname "$BINARY_PATH")
APP_BUNDLE="$BINARY_DIR/rmmagent.app"

echo -e "${YELLOW}Creating app bundle structure at: $APP_BUNDLE${NC}"

# Create bundle structure
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

echo -e "${GREEN}✓ Created bundle directories${NC}"

# Copy binary
echo -e "${YELLOW}Copying binary to app bundle${NC}"
cp "$BINARY_PATH" "$APP_BUNDLE/Contents/MacOS/rmmagent"
chmod +x "$APP_BUNDLE/Contents/MacOS/rmmagent"
echo -e "${GREEN}✓ Binary copied${NC}"

# Generate Info.plist from template
TEMPLATE_FILE="$SCRIPT_DIR/Info.plist.template"
PLIST_FILE="$APP_BUNDLE/Contents/Info.plist"

if [ ! -f "$TEMPLATE_FILE" ]; then
    echo -e "${RED}Error: Info.plist.template not found at: $TEMPLATE_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}Generating Info.plist with version ${VERSION}${NC}"

# Replace {{VERSION}} placeholder with actual version
sed "s/{{VERSION}}/$VERSION/g" "$TEMPLATE_FILE" > "$PLIST_FILE"

# Validate plist
if plutil -lint "$PLIST_FILE" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Info.plist created and validated${NC}"
else
    echo -e "${RED}✗ Info.plist validation failed${NC}"
    exit 1
fi

# Show bundle info
echo ""
echo -e "${BLUE}App Bundle Information:${NC}"
echo "  Bundle path: $APP_BUNDLE"
echo "  Bundle identifier: com.amidaware.rmmagent"
echo "  Version: $VERSION"
echo "  Executable: rmmagent"

# Verify bundle structure
if [ -f "$APP_BUNDLE/Contents/MacOS/rmmagent" ] && \
   [ -f "$APP_BUNDLE/Contents/Info.plist" ]; then
    echo ""
    echo -e "${GREEN}===========================================================${NC}"
    echo -e "${GREEN}App bundle created successfully!${NC}"
    echo -e "${GREEN}===========================================================${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Sign the bundle:"
    echo "     export MACOS_SIGN_CERT=\"Developer ID Application: Your Name (TEAM)\""
    echo "     ./scripts/macos/sign-macos.sh"
    echo ""
    echo "  2. Notarize the bundle:"
    echo "     ./scripts/macos/notarize-macos.sh"
    echo ""
    echo "  3. Staple the notarization (optional):"
    echo "     xcrun stapler staple \"$APP_BUNDLE\""
else
    echo -e "${RED}Error: Bundle creation incomplete${NC}"
    exit 1
fi
