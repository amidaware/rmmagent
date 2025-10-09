#!/bin/bash
# Generate Windows resource files with icon embedded
# This script should be run before building the Windows executable

set -e

echo "Generating Windows resource files..."

# Check if goversioninfo is installed
if ! command -v goversioninfo &> /dev/null; then
    echo "Installing goversioninfo..."
    go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
fi

# Use full path to goversioninfo if not in PATH
GOVERSIONINFO="goversioninfo"
if ! command -v goversioninfo &> /dev/null; then
    GOVERSIONINFO="$HOME/go/bin/goversioninfo"
fi

# Generate 64-bit resource file
echo "Generating resource.syso (64-bit)..."
$GOVERSIONINFO -64 versioninfo.json

# Generate 32-bit resource file
echo "Generating resource_386.syso (32-bit)..."
$GOVERSIONINFO -o resource_386.syso versioninfo.json

echo "Resource files generated successfully!"
echo "  - resource.syso (amd64)"
echo "  - resource_386.syso (386)"
