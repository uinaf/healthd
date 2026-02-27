#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Usage: scripts/release.sh v0.X.0}"
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid version: $VERSION (expected vX.Y.Z)"; exit 1; }

DIR="$(mktemp -d)"
trap 'rm -rf "$DIR"' EXIT

echo "==> Building binaries..."
GOOS=darwin GOARCH=arm64 go build -o "$DIR/healthd_arm64" .
GOOS=darwin GOARCH=amd64 go build -o "$DIR/healthd_amd64" .

echo "==> Creating tarballs..."
for arch in arm64 amd64; do
  cp "$DIR/healthd_${arch}" "$DIR/healthd"
  tar czf "$DIR/healthd_${VERSION}_darwin_${arch}.tar.gz" -C "$DIR" healthd
  rm "$DIR/healthd"
done

echo "==> Computing checksums..."
(cd "$DIR" && shasum -a 256 healthd_${VERSION}_darwin_*.tar.gz > checksums.txt)
cat "$DIR/checksums.txt"

echo "==> Tagging ${VERSION}..."
git tag -a "$VERSION" -m "Release ${VERSION}"
git push origin "$VERSION"

echo "==> Creating GitHub release..."
gh release create "$VERSION" \
  "$DIR/healthd_${VERSION}_darwin_arm64.tar.gz" \
  "$DIR/healthd_${VERSION}_darwin_amd64.tar.gz" \
  "$DIR/checksums.txt" \
  --title "$VERSION" \
  --generate-notes

echo "==> Updating homebrew formula..."
ARM_SHA=$(grep arm64 "$DIR/checksums.txt" | awk '{print $1}')
AMD_SHA=$(grep amd64 "$DIR/checksums.txt" | awk '{print $1}')
VER_NUM="${VERSION#v}"

FORMULA_PATH="${HOMEBREW_TAP:-$HOME/projects/homebrew-tap}/Formula/healthd.rb"
sed -i '' \
  -e "s/version \".*\"/version \"${VER_NUM}\"/" \
  -e "s|download/v[^/]*/healthd_v[^\"]*_darwin_arm64|download/${VERSION}/healthd_${VERSION}_darwin_arm64|" \
  -e "s|download/v[^/]*/healthd_v[^\"]*_darwin_amd64|download/${VERSION}/healthd_${VERSION}_darwin_amd64|" \
  "$FORMULA_PATH"

# Update sha256 values using awk (match tarball URL line, update next sha256 line)
awk -v arm="$ARM_SHA" -v amd="$AMD_SHA" '
  /arm64\.tar\.gz/ { found_arm=1 }
  found_arm && /sha256/ { sub(/"[a-f0-9]+"/, "\"" arm "\""); found_arm=0 }
  /amd64\.tar\.gz/ { found_amd=1 }
  found_amd && /sha256/ { sub(/"[a-f0-9]+"/, "\"" amd "\""); found_amd=0 }
  { print }
' "$FORMULA_PATH" > "${FORMULA_PATH}.tmp" && mv "${FORMULA_PATH}.tmp" "$FORMULA_PATH"

cd "$(dirname "$FORMULA_PATH")/.."
git add Formula/healthd.rb
git commit -m "healthd: bump to ${VERSION}"
git push

echo "==> Done! Run 'brew upgrade healthd' to verify."
