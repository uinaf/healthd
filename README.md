![healthd — pluggable host health check daemon in Go.](https://uinaf.dev/og/banner/healthd.png)

# uinaf/healthd

Lightweight host health daemon for one machine: scheduled checks, machine-readable status, and alerts on fail/recover transitions.

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
healthd init
healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
healthd status --config ~/.config/healthd/config.toml
healthd status --config ~/.config/healthd/config.toml --watch
healthd notify test --config ~/.config/healthd/config.toml
healthd run --config ~/.config/healthd/config.toml
```

`healthd run` is meant to be supervised (process-compose, systemd, launchd).

## Example notify config

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

More complete host profiles: [examples/current-host.toml](examples/current-host.toml).

## Docs

- [Architecture](docs/ARCHITECTURE.md): components, lifecycle, design choices
- [Contributing](CONTRIBUTING.md): setup, verify, pull requests
- [Releases](docs/RELEASES.md): Conventional Commits publish path
- [Security](https://github.com/uinaf/healthd/security/policy): private vulnerability reporting

## License

[MIT](LICENSE)
