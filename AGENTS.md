# healthd

Local host health-check daemon. Runs checks, alerts on transitions. See `docs/ARCHITECTURE.md`.

## Stack
- Go 1.26+ (toolchain pin in `.tool-versions`), Cobra CLI, TOML config
- TUI: charmbracelet/bubbletea + lipgloss
- Notifications: ntfy, webhook, command backends

## Commands
```bash
go run ./cmd/verify                     # full gate (fmt + lint + test + 80% coverage)
go build ./...                          # build
go test ./...                           # quick: all tests, no gate
go build -o ~/.local/bin/healthd .      # install to ~/.local/bin
```

## Releases
Automated on push to `main`. CI verifies, `semantic-release` decides the version from Conventional Commits and tags + drafts the GitHub Release, then GoReleaser builds darwin/arm64 + darwin/amd64 tarballs and updates the formula in `uinaf/homebrew-tap`. No manual script — write `feat:` / `fix:` / `feat!:` commits and the bump happens. Use `[skip ci]` in the message to skip a release for a given push.

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
- Keep each branch focused on one concern
