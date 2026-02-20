# healthd

Pluggable host health-check daemon written in Go.

## Install (binary-first)

Use prebuilt release binaries by default.

### macOS (copy-paste)

```bash
curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash
# optional pinned version:
# curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash -s -- v0.1.0
```

Manual install (if you prefer):

```bash
VERSION="vX.Y.Z"
OS="darwin"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH"; exit 1 ;;
esac

ARTIFACT="healthd_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/uinaf/healthd/releases/download/${VERSION}"

curl -fL "${BASE_URL}/${ARTIFACT}" -o "${ARTIFACT}"
curl -fL "${BASE_URL}/checksums.txt" -o checksums.txt
grep "  ${ARTIFACT}$" checksums.txt | shasum -a 256 -c -
tar -xzf "${ARTIFACT}"
install -m 0755 healthd /usr/local/bin/healthd
healthd --version
```

### Source build fallback

```bash
go install github.com/uinaf/healthd@latest
healthd --version
```

## Quickstart

1. Copy baseline config:

```bash
mkdir -p ~/.config/healthd
cp examples/current-host.toml ~/.config/healthd/config.toml
```

2. Validate + run one-shot checks:

```bash
healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
```

## Notifications (ntfy default + local-log fallback)

Use ntfy for easiest phone push notifications, and keep a local command notifier as backup.

### Config snippet

```toml
[notify]
cooldown = "5m"

[[notify.backend]]
name = "ntfy-phone"
type = "ntfy"
topic = "replace-with-strong-random-topic"

[[notify.backend]]
name = "local-log"
type = "command"
command = "logger -t healthd-alert"
timeout = "5s"
```

### Choose a strong random topic

```bash
openssl rand -hex 16
# example: 6f1a6b3f6a4a89e4d117e8a355ec21d0
```

Then set `topic = "<that-random-value>"` in config.

### Verify notifier wiring

```bash
healthd notify test --config ~/.config/healthd/config.toml --backend ntfy-phone
# backup path:
healthd notify test --config ~/.config/healthd/config.toml --backend local-log
```

## Local verification

Run the same checks used in CI:

```bash
go run ./cmd/verify
```
