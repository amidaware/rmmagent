#!/bin/bash

# Personal wrapper script for signing and notarizing rmmagent
# This template should be copied to bin/ and customized with your credentials
#
# Setup:
#   1. Copy this template to bin/:
#      cp scripts/templates-for-bin/sign-and-notarize-rmmagent-template.sh bin/
#
#   2. Edit the CERT variable below with your Developer ID Application certificate
#
#   3. Configure the notarization keychain profile (one-time):
#      xcrun notarytool store-credentials "rmmagent-notary" \
#          --apple-id "your@email.com" \
#          --team-id "YOUR_TEAM_ID" \
#          --password "xxxx-xxxx-xxxx-xxxx"
#
#      Note: Password should be an app-specific password from:
#      https://appleid.apple.com → Security → App-Specific Passwords
#
#   4. Run this script after building:
#      ./bin/sign-and-notarize-rmmagent-template.sh

set -e

# ============================================================================
# CONFIGURATION - UPDATE THESE VALUES
# ============================================================================

# Your Developer ID Application certificate
# Find it with: security find-identity -v -p codesigning
CERT="Developer ID Application: Your Name (TEAMID)"

# Control which operations to perform
DO_SIGN=true
DO_NOTARIZE=true
DO_STAPLE=false

# ============================================================================
# END CONFIGURATION
# ============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect repository root (works whether run from bin/ or templates-for-bin/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if we're in bin/ or templates-for-bin/
if [[ "$SCRIPT_DIR" == */templates-for-bin ]]; then
    REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
elif [[ "$SCRIPT_DIR" == */bin ]]; then
    REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    # Try to find git root
    REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || echo "$SCRIPT_DIR")"
fi

echo -e "${BLUE}===========================================================${NC}"
echo -e "${BLUE}rmmagent Sign and Notarize${NC}"
echo -e "${BLUE}===========================================================${NC}"
echo ""
echo "Repository root: $REPO_ROOT"
echo "Certificate: $CERT"
echo ""

# Check if certificate is the template default
if [ "$CERT" = "Developer ID Application: Your Name (TEAMID)" ]; then
    echo -e "${RED}Error: Please update the CERT variable in this script${NC}"
    echo ""
    echo "Find your certificate with:"
    echo "  security find-identity -v -p codesigning"
    echo ""
    echo "Then update this script and set CERT to your certificate name."
    exit 1
fi

# Export certificate for the signing script
export MACOS_SIGN_CERT="$CERT"

# Export control flags for the pipeline script
export DO_SIGN
export DO_NOTARIZE
export DO_STAPLE

# Run the build pipeline
"$REPO_ROOT/scripts/macos/build-pipeline-macos.sh"

# Summary
echo ""
echo -e "${GREEN}===========================================================${NC}"
echo -e "${GREEN}Operations Summary${NC}"
echo -e "${GREEN}===========================================================${NC}"
echo ""
[ "$DO_SIGN" = "true" ] && echo -e "${GREEN}✓ Code Signing${NC}" || echo "- Code Signing (skipped)"
[ "$DO_NOTARIZE" = "true" ] && echo -e "${GREEN}✓ Notarization${NC}" || echo "- Notarization (skipped)"
[ "$DO_STAPLE" = "true" ] && echo -e "${GREEN}✓ Stapling${NC}" || echo "- Stapling (skipped)"
echo ""
echo -e "${GREEN}All operations completed successfully!${NC}"
