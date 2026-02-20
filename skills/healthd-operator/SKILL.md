---
name: healthd-operator
description: Operate healthd safely on a host with binary-first install, ntfy-first notifications, daemon lifecycle, and rollback steps.
---

# healthd Operator

## Default install flow (binary-first)

```bash
VERSION="vX.Y.Z"
OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH"; exit 1 ;;
esac

ARTIFACT="healthd_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/uinaf/healthd/releases/download/${VERSION}"

curl -fL "${BASE_URL}/${ARTIFACT}" -o "${ARTIFACT}"
curl -fL "${BASE_URL}/checksums.txt" -o checksums.txt
grep "  ${ARTIFACT}$" checksums.txt | shasum -a 256 -c -
tar -xzf "${ARTIFACT}"
install -m 0755 healthd /usr/local/bin/healthd
```

Source fallback:

```bash
go install github.com/uinaf/healthd@latest
```

## Notifications (default easiest path: ntfy)

Use ntfy for phone push alerts, with local-log as backup.

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

Generate a strong topic:

```bash
openssl rand -hex 16
```

Test notifier delivery:

```bash
healthd notify test --config ~/.config/healthd/config.toml --backend ntfy-phone
# backup notifier:
healthd notify test --config ~/.config/healthd/config.toml --backend local-log
```
