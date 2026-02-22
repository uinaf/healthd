# healthd

`healthd` is a lightweight host health daemon for one machine.

It exists to replace fragile cron-only checks with a single, reliable loop that:
- runs checks on a schedule
- returns machine-readable status (`--json`)
- sends alerts on fail/recover transitions

## Motivation

Most setups end up with scattered shell scripts, ad-hoc cron jobs, and noisy alerts.
`healthd` gives you one place to define checks, one output format, and pluggable notifications.

## Good use cases

- Monitor local daemons/services (trading bots, workers, sidecars)
- Catch machine drift (disk, network, gateway, Docker/Colima)
- Replace many small cron probes with one structured checker
- Build automations on top of JSON output

## Install

### Homebrew

```bash
brew tap uinaf/tap
brew install healthd
```

### Direct install

```bash
curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash
# optional pinned version:
# curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash -s -- v0.1.0
healthd --version
```

## Quickstart

```bash
# 1) create starter config
healthd init

# 2) validate config
healthd validate --config ~/.config/healthd/config.toml

# 3) run checks once
healthd check --config ~/.config/healthd/config.toml

# 4) test notifier
healthd notify test --config ~/.config/healthd/config.toml

# 5) install daemon mode
healthd daemon install --config ~/.config/healthd/config.toml
```

On macOS, daemon install writes an explicit LaunchAgent `PATH`
(`/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`) so command checks can find Homebrew binaries under launchd.

## Example notifier config

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

## Testing

Required CI checks:

```bash
go test ./...
go test ./integration/... -v
go test ./e2e/cli/... -v
```

Optional host-level macOS daemon e2e:

```bash
HEALTHD_HOST_E2E=1 go test -tags=hoste2e ./e2e/host/... -v
```

More details: `docs/testing.md`.
