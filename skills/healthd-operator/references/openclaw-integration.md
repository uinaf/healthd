# OpenClaw integration: healthd operator flow

Use this flow when asked to operate `healthd` on a local host from an OpenClaw session.

## 1) Detect context first

```bash
uname -a
whoami
which healthd || echo "healthd not in PATH"
printenv HEALTHD_CONFIG || true
# Detect supervisor (any of these may apply, none guaranteed):
process-compose process list 2>/dev/null | grep healthd || true
launchctl list 2>/dev/null | grep healthd || true
systemctl --user status healthd 2>/dev/null || true
```

Ask for confirmation before changing supervisor state or replacing existing scheduler jobs.

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

## 5) Run continuously

```bash
# Foreground (debug); Ctrl-C to stop:
healthd daemon run --config ~/.config/healthd/config.toml

# Production: have your supervisor (process-compose / systemd / launchd)
# invoke the same command. Lifecycle (install/start/stop/logs) is the
# supervisor's responsibility, not healthd's.
```

## 6) Notification smoke test

```bash
healthd notify test --config ~/.config/healthd/config.toml
# or specific backend:
healthd notify test --config ~/.config/healthd/config.toml --backend ops-webhook
```

## 7) Inspect alert history

```bash
tail -n 20 ~/.local/state/healthd/alerts.log
# or live:
healthd status --watch --config ~/.config/healthd/config.toml
```

## 8) Rollback path (confirm first)

```bash
# Ask the supervisor to stop healthd before restoring prior config/scheduler.
# Restore old scheduler only if user confirms the exact command/job.
```

## Suggested operator response template

- **State detected:** binary path, config path, supervisor (if any), recent transitions from alerts.log.
- **Actions run:** exact commands + outcomes.
- **Risks:** what was not changed without confirmation.
- **Next step:** whether to keep running, tune checks, or rollback.
