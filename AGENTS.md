# healthd

Local host health-check daemon. Runs checks, alerts on transitions. See `docs/ARCHITECTURE.md`.

## Stack
- Go 1.26, Cobra CLI, TOML config
- TUI: charmbracelet/bubbletea + lipgloss
- Notifications: ntfy, command backends

## Commands
```bash
go run ./cmd/verify                     # full gate (fmt + lint + test + 80% coverage)
go build ./...                          # build
go test ./...                           # quick: all tests, no gate
go build -o ~/.local/bin/healthd .      # install to ~/.local/bin
scripts/release.sh vX.Y.Z               # full release + brew update
```

## Env
- `HEALTHD_CONFIG` — override config path (also via `--config`); useful for per-worktree isolation

## Hooks
```bash
git config core.hooksPath .git-hooks    # one-time: enable pre-push verifier
```

## Running
```bash
healthd check --config ~/.config/healthd/config.toml       # one-shot
healthd status --config ~/.config/healthd/config.toml       # TUI
healthd status --config ~/.config/healthd/config.toml -w    # live dashboard
healthd run --config ~/.config/healthd/config.toml          # long-running loop (managed by process-compose)
```

## Paths
- **Binary:** `~/.local/bin/healthd`
- **Config:** `~/.config/healthd/config.toml`
- **State:** `~/.local/state/healthd/` (alerts.log)

## Conventions
- Build in small vertical slices aligned to issues
- Prefer stacked PRs with Graphite for dependent work
- Parse config into typed structs; reject unknown keys
- No auto-remediation in v1 (detect/report only)
- Keep packages focused and testable
- Do not add unrelated changes in the same branch
