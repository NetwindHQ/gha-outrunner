#!/bin/bash
set -euo pipefail

# Updates the Homebrew formula in NetwindHQ/homebrew-tap with new version and checksums.
# Expects: VERSION, CHECKSUMS_FILE environment variables, and GH_TOKEN for authentication.

if [ -z "${VERSION:-}" ] || [ -z "${CHECKSUMS_FILE:-}" ]; then
  echo "Usage: VERSION=vx.y.z CHECKSUMS_FILE=path/to/checksums.txt $0"
  exit 1
fi

# Strip leading 'v' for the formula version field
VERSION="${VERSION#v}"

SHA_DARWIN_ARM64=$(grep "darwin_arm64.tar.gz" "$CHECKSUMS_FILE" | awk '{print $1}')
SHA_LINUX_AMD64=$(grep "linux_amd64.tar.gz" "$CHECKSUMS_FILE" | awk '{print $1}')
SHA_LINUX_ARM64=$(grep "linux_arm64.tar.gz" "$CHECKSUMS_FILE" | awk '{print $1}')

echo "Updating formula to $VERSION"
echo "  darwin/arm64: $SHA_DARWIN_ARM64"
echo "  linux/amd64:  $SHA_LINUX_AMD64"
echo "  linux/arm64:  $SHA_LINUX_ARM64"

# Fetch current formula
RESPONSE=$(gh api repos/NetwindHQ/homebrew-tap/contents/Formula/outrunner.rb)
FILE_SHA=$(echo "$RESPONSE" | jq -r '.sha')
FORMULA=$(echo "$RESPONSE" | jq -r '.content' | base64 -d)

# Update version
FORMULA=$(echo "$FORMULA" | sed "s/version \".*\"/version \"$VERSION\"/")

# Update checksums in order: darwin_arm64, linux_amd64, linux_arm64
FORMULA=$(echo "$FORMULA" | awk -v sha="$SHA_DARWIN_ARM64" '/sha256/ && !a++ {sub(/sha256 "[a-f0-9]+"/, "sha256 \"" sha "\"")} 1')
FORMULA=$(echo "$FORMULA" | awk -v sha="$SHA_LINUX_AMD64" '/sha256/ && ++c==2 {sub(/sha256 "[a-f0-9]+"/, "sha256 \"" sha "\"")} 1')
FORMULA=$(echo "$FORMULA" | awk -v sha="$SHA_LINUX_ARM64" '/sha256/ && ++c==3 {sub(/sha256 "[a-f0-9]+"/, "sha256 \"" sha "\"")} 1')

# Push updated formula
CONTENT=$(echo "$FORMULA" | base64 | tr -d '\n')
gh api -X PUT repos/NetwindHQ/homebrew-tap/contents/Formula/outrunner.rb \
  --field message="Update outrunner to $VERSION" \
  --field sha="$FILE_SHA" \
  --field content="$CONTENT"

echo "Formula updated successfully"
