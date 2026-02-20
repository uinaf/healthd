# OpenClaw integration: healthd operator flow

Use this flow when asked to operate `healthd` on a local host from an OpenClaw session.

## 1) Detect context first

```bash
uname -a
whoami
which healthd || echo "healthd not in PATH"
healthd daemon status || true
printenv HEALTHD_CONFIG || true
```

Ask for confirmation before changing daemon install state or replacing existing scheduler jobs.

## 2) Build/install binary

```bash
cd ~/projects/healthd
go build -o ./bin/healthd .
./bin/healthd --help
```

Optional user-level install:

```bash
go install .
healthd --help
```

## 3) Baseline config bootstrap

```bash
healthd init
# custom path:
# healthd init --config /path/to/config.toml
# overwrite existing file only if intended:
# healthd init --config /path/to/config.toml --force
$EDITOR ~/.config/healthd/config.toml
```

## 4) Validate + one-shot checks

```bash
healthd validate --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml
healthd check --config ~/.config/healthd/config.toml --json
```

## 5) Daemon lifecycle

```bash
healthd daemon install --config ~/.config/healthd/config.toml
healthd daemon status
healthd daemon logs --lines 100
```

## 6) Notification smoke test

```bash
healthd notify test --config ~/.config/healthd/config.toml
# or specific backend:
healthd notify test --config ~/.config/healthd/config.toml --backend ops-webhook
```

## 7) Rollback path (confirm first)

```bash
healthd daemon uninstall
# restore old scheduler only if user confirms the exact command/job
```

## Suggested operator response template

- **State detected:** binary path, config path, daemon status.
- **Actions run:** exact commands + outcomes.
- **Risks:** what was not changed without confirmation.
- **Next step:** whether to keep daemon, tune checks, or rollback.
