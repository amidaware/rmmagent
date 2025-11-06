#!/bin/bash

# Build script for macOS universal binary (AMD64 + ARM64)
# This script builds the rmmagent for both architectures and combines them using lipo

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the root directory of the project
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Output directories
AMD64_OUTPUT="${PROJECT_ROOT}/build/Output/macos/amd64"
ARM64_OUTPUT="${PROJECT_ROOT}/build/Output/macos/arm64"
UNIVERSAL_OUTPUT="${PROJECT_ROOT}/build/Output/macos/universal"

# Binary name
BINARY_NAME="rmmagent"

echo -e "${GREEN}Starting macOS universal binary build...${NC}"
echo "Project root: ${PROJECT_ROOT}"

# Create output directories
echo -e "${YELLOW}Creating output directories...${NC}"
mkdir -p "${AMD64_OUTPUT}"
mkdir -p "${ARM64_OUTPUT}"
mkdir -p "${UNIVERSAL_OUTPUT}"

# Build AMD64
echo -e "${YELLOW}Building AMD64 binary...${NC}"
cd "${PROJECT_ROOT}"
GOOS=darwin GOARCH=amd64 go build -o "${AMD64_OUTPUT}/${BINARY_NAME}" -ldflags="-s -w" .
echo -e "${GREEN}✓ AMD64 build complete: ${AMD64_OUTPUT}/${BINARY_NAME}${NC}"

# Build ARM64
echo -e "${YELLOW}Building ARM64 binary...${NC}"
cd "${PROJECT_ROOT}"
GOOS=darwin GOARCH=arm64 go build -o "${ARM64_OUTPUT}/${BINARY_NAME}" -ldflags="-s -w" .
echo -e "${GREEN}✓ ARM64 build complete: ${ARM64_OUTPUT}/${BINARY_NAME}${NC}"

# Create universal binary with lipo
echo -e "${YELLOW}Creating universal binary with lipo...${NC}"
lipo -create \
    "${AMD64_OUTPUT}/${BINARY_NAME}" \
    "${ARM64_OUTPUT}/${BINARY_NAME}" \
    -output "${UNIVERSAL_OUTPUT}/${BINARY_NAME}"
echo -e "${GREEN}✓ Universal binary created: ${UNIVERSAL_OUTPUT}/${BINARY_NAME}${NC}"

# Verify the universal binary
echo -e "${YELLOW}Verifying universal binary...${NC}"
lipo -info "${UNIVERSAL_OUTPUT}/${BINARY_NAME}"

# Show file sizes
echo -e "${YELLOW}Binary sizes:${NC}"
ls -lh "${AMD64_OUTPUT}/${BINARY_NAME}" | awk '{print "AMD64:     " $5}'
ls -lh "${ARM64_OUTPUT}/${BINARY_NAME}" | awk '{print "ARM64:     " $5}'
ls -lh "${UNIVERSAL_OUTPUT}/${BINARY_NAME}" | awk '{print "Universal: " $5}'

echo -e "${GREEN}Build complete!${NC}"

# Optional: Code signing integration
if [ -n "$MACOS_SIGN_CERT" ]; then
    echo ""
    echo -e "${YELLOW}Code signing certificate detected: $MACOS_SIGN_CERT${NC}"
    echo -e "${YELLOW}Running code signing...${NC}"
    "${SCRIPT_DIR}/sign-macos.sh"
else
    echo ""
    echo -e "${YELLOW}Tip: To automatically sign binaries after building, set:${NC}"
    echo "  export MACOS_SIGN_CERT=\"Developer ID Application: Your Name (TEAMID)\""
fi
