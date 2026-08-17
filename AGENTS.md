# healthd

Local host health-check daemon. Runs checks and alerts on fail/recover transitions.

## Verify

```bash
go run ./cmd/verify   # fmt + lint + test + coverage gate (80% total, 70%/package)
go test ./...
```

## Build

```bash
go build ./...
go build -o ~/.local/bin/healthd .
```

## Boundaries

- Detect and report only; no auto-remediation
- Parse config into typed structs; reject unknown keys
- Keep packages focused and testable

## Config and state

| Item | Path / override |
|---|---|
| Config | `~/.config/healthd/config.toml` (`--config` or `HEALTHD_CONFIG`) |
| Alerts | `~/.local/state/healthd/alerts.log` |

## Hooks

```bash
git config core.hooksPath .git-hooks
```

## Docs

- [Architecture](docs/ARCHITECTURE.md): components and check lifecycle
- [Contributing](CONTRIBUTING.md): setup, verify, pull requests
- [Releases](docs/RELEASES.md): Conventional Commits publish path
- [Security](https://github.com/uinaf/healthd/security/policy): private vulnerability reporting
- [Operator skill](skills/healthd-operator/SKILL.md): install and operate on a host
