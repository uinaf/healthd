# healthd

`healthd` is a small CLI + daemon for host health checks.
It validates config, runs checks on demand, and can run continuously as a macOS LaunchAgent.
It can send alerts to `ntfy` and a local fallback command.

## Install

Choose one install method:

### Homebrew

```bash
brew tap uinaf/tap
brew install healthd
healthd --version
```

### Direct install script

```bash
curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash
healthd --version
```

### Build from source

```bash
go install github.com/uinaf/healthd@latest
healthd --version
```

## Quick start

```bash
healthd init
healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
```

## Notifications

Use `ntfy` for push alerts, plus a local command backend as fallback.

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

Generate a topic:

```bash
openssl rand -hex 16
```

Test both backends:

```bash
healthd notify test --config ~/.config/healthd/config.toml --backend ntfy-phone
healthd notify test --config ~/.config/healthd/config.toml --backend local-log
```

## Daemon basics (macOS)

```bash
healthd daemon install --config ~/.config/healthd/config.toml
healthd daemon status
healthd daemon logs --lines 100
healthd daemon uninstall
```
