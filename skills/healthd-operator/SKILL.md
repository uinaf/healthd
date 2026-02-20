---
name: healthd-operator
description: Operate healthd safely on a local host: detect environment, install/build binary, bootstrap config, validate and run one-shot checks, install/manage daemon, test notifications, and rollback/uninstall. Use when asked to deploy healthd, troubleshoot health checks, migrate from cron, inspect daemon/log status, or verify alert delivery.
---

# healthd Operator

Use this skill for practical host operations. Prefer reversible steps and show commands before running them.

## Safety rules

- Do **not** uninstall services, overwrite configs, or remove cron jobs without explicit confirmation.
- Before daemon changes, capture current state (`healthd daemon status`, existing cron entries, config path).
- Keep edits scoped to requested host/application checks.

## Workflow

1. **Detect environment/context**
   - Confirm OS/user, repo vs installed binary, config path (`--config` / `HEALTHD_CONFIG` / default).
   - Check if daemon is already installed: `healthd daemon status`.
2. **Install/build binary**
   - For local dev: `go build -o ./bin/healthd .`
   - For user install: `go install .` (or move built binary into PATH).
3. **Generate baseline config**
   - Start from `examples/current-host.toml` and adapt checks/backends.
   - Write config to `~/.config/healthd/config.toml` unless user specifies another path.
4. **Validate + one-shot checks**
   - `healthd validate --config <path>`
   - `healthd check --config <path>` and `--json` when automation output is needed.
5. **Daemon install/status/logs**
   - Install: `healthd daemon install --config <path>`
   - Verify: `healthd daemon status`
   - Inspect logs: `healthd daemon logs --lines 100`
6. **Notification test**
   - `healthd notify test --config <path>`
   - Optionally narrow with `--backend <name-or-type>`.
7. **Rollback/uninstall**
   - `healthd daemon uninstall`
   - Restore prior scheduler (for example cron) only after confirmation.

## Troubleshooting checklist

- Config parse/validation fails: run `healthd validate --config <path>` and fix unknown keys/types.
- No checks run: verify `[[check]]` entries and any `--only/--group` filters.
- Daemon not running: check `healthd daemon status` and `healthd daemon logs --lines 100`.
- Alerts not sent: verify notifier blocks and run `healthd notify test --backend <name>`.
- Command checks flaky: increase `timeout`, run command manually, then retest with `healthd check`.

## References

- Config snippets: `references/config-examples.md`
- OpenClaw operator flow: `references/openclaw-integration.md`
