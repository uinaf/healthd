#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-latest}"
if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL https://api.github.com/repos/uinaf/healthd/releases/latest | jq -r .tag_name)"
fi

OS="$(uname | tr '[:upper:]' '[:lower:]')"
if [[ "$OS" != "darwin" ]]; then
  echo "Unsupported OS: $OS (healthd release artifacts currently target macOS only)" >&2
  exit 1
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64) ARCH="arm64" ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac

ARTIFACT="healthd_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/uinaf/healthd/releases/download/${VERSION}"

curl -fL "${BASE_URL}/${ARTIFACT}" -o "${ARTIFACT}"
curl -fL "${BASE_URL}/checksums.txt" -o checksums.txt
grep "  ${ARTIFACT}$" checksums.txt | shasum -a 256 -c -
tar -xzf "${ARTIFACT}"

install -m 0755 healthd /usr/local/bin/healthd
rm -f "${ARTIFACT}" checksums.txt healthd

echo "Installed healthd ${VERSION} to /usr/local/bin/healthd"
healthd --version || true
