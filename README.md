# healthd

Pluggable host health-check daemon written in Go.

## Install

Pick any install method:

### Homebrew

```bash
brew tap uinaf/tap
brew install healthd
```

### Direct install (script)

```bash
curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash
# optional pinned version:
# curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash -s -- v0.1.0
healthd --version
```

### Source build

```bash
go install github.com/uinaf/healthd@latest
healthd --version
```

## First run

```bash
healthd init
# optional custom path:
# healthd init --config /path/to/config.toml
# overwrite existing config:
# healthd init --force

healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
```

## Notifications

Use `ntfy` for the easiest phone push path.

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

### Validate, check, then test notify

```bash
healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
healthd notify test --config ~/.config/healthd/config.toml --backend ntfy-phone
# backup path:
healthd notify test --config ~/.config/healthd/config.toml --backend local-log
```

## Local verification

Run the same checks used in CI:

```bash
go run ./cmd/verify
```
