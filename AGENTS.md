# healthd

Local host health-check daemon. Runs checks, alerts on transitions. See `docs/ARCHITECTURE.md`.

## Stack
- Go 1.24, Cobra CLI, TOML config
- TUI: charmbracelet/bubbletea + lipgloss
- Notifications: ntfy, command backends

## Commands
```bash
go build ./...           # build
go test ./...            # all tests
go vet ./...             # lint
go test -cover ./...     # coverage (80% gate)
make install             # build → ~/.local/bin/healthd
scripts/release.sh vX.Y.Z  # full release + brew update
```

## Running
```bash
healthd check --config ~/.config/healthd/config.toml       # one-shot
healthd status --config ~/.config/healthd/config.toml       # TUI
healthd status --config ~/.config/healthd/config.toml -w    # live dashboard
healthd daemon install --config ~/.config/healthd/config.toml  # launchd
```

## Paths
- **Binary:** `~/.local/bin/healthd`
- **Config:** `~/.config/healthd/config.toml`
- **State:** `~/.local/state/healthd/` (alerts.log)
- **LaunchAgent:** `com.uinaf.healthd`

## Conventions
- Build in small vertical slices aligned to issues
- Prefer stacked PRs with Graphite for dependent work
- Parse config into typed structs; reject unknown keys
- No auto-remediation in v1 (detect/report only)
- Keep packages focused and testable
- Do not add unrelated changes in the same branch
