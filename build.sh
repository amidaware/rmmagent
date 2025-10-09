#!/bin/bash
# Build script for Tactical RMM Agent
# This script builds the agent executable for various platforms

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
OS=""
ARCH=""
OUTPUT=""

show_help() {
    echo -e "${CYAN}Tactical RMM Agent - Build Script${NC}"
    echo ""
    echo "Usage: ./build.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -o, --os <os>          Target OS (windows, linux, darwin). Default: auto-detect"
    echo "  -a, --arch <arch>      Target architecture (amd64, 386, arm64, arm). Default: amd64"
    echo "  -h, --help             Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./build.sh                              # Build for current OS (amd64)"
    echo "  ./build.sh -o windows                   # Build for Windows (amd64)"
    echo "  ./build.sh -o linux -a arm64            # Build for Linux ARM64"
    echo "  ./build.sh -o darwin -a arm64           # Build for macOS Apple Silicon"
    echo ""
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--os)
            OS="$2"
            shift 2
            ;;
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        -h|--help)
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

# Auto-detect OS if not specified
if [ -z "$OS" ]; then
    case "$(uname -s)" in
        Linux*)     OS=linux;;
        Darwin*)    OS=darwin;;
        MINGW*|MSYS*|CYGWIN*)    OS=windows;;
        *)          OS=linux;;
    esac
fi

# Default architecture
if [ -z "$ARCH" ]; then
    ARCH="amd64"
fi

# Determine output filename
case "$OS" in
    windows)
        if [ "$ARCH" = "amd64" ]; then
            OUTPUT="tacticalrmm.exe"
        else
            OUTPUT="tacticalrmm-${ARCH}.exe"
        fi
        ;;
    linux|darwin)
        OUTPUT="tacticalrmm-${OS}-${ARCH}"
        ;;
    *)
        echo -e "${RED}Unknown OS: $OS${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}Building Tactical RMM Agent...${NC}"
echo -e "  ${YELLOW}OS: $OS${NC}"
echo -e "  ${YELLOW}Architecture: $ARCH${NC}"
echo -e "  ${YELLOW}Output: $OUTPUT${NC}"
echo ""

# Build
export CGO_ENABLED=0
export GOOS=$OS
export GOARCH=$ARCH

if go build -ldflags "-s -w" -o "$OUTPUT"; then
    echo -e "${GREEN}Build completed successfully!${NC}"
    echo -e "${CYAN}Output file: $OUTPUT${NC}"
    
    # Show file size
    if command -v du &> /dev/null; then
        FILE_SIZE=$(du -h "$OUTPUT" | cut -f1)
        echo -e "${CYAN}File size: $FILE_SIZE${NC}"
    fi
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi
