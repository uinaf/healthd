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
   - Preferred direct install: `curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash`
   - Optional pinned version: `curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash -s -- v0.1.0`
   - Source fallback for local dev: `go build -o ./bin/healthd .` or `go install .`.
3. **First-run config bootstrap**
   - Run `healthd init` (default: `~/.config/healthd/config.toml`).
   - Use `healthd init --config <path>` for non-default location.
   - If file exists, use `healthd init --config <path> --force` only with explicit confirmation.
4. **Quick validation and smoke checks**
   - `healthd validate --config <path>`
   - `healthd check --config <path>` (`--json` for automation output).
5. **Quick notify test (ntfy easiest path)**
   - Add an `ntfy` backend topic in config if none exists.
   - Run `healthd notify test --config <path> --backend <name-or-type>`.
6. **Daemon install/status/logs**
   - Install: `healthd daemon install --config <path>`
   - Verify: `healthd daemon status`
   - Inspect logs: `healthd daemon logs --lines 100`
7. **Rollback/uninstall**
   - `healthd daemon uninstall`
   - Restore prior scheduler (for example cron) only after confirmation and with exact command captured in session notes.

## Troubleshooting checklist

- Config parse/validation fails: run `healthd validate --config <path>` and fix unknown keys/types.
- `healthd init` fails with existing file: rerun with `--force` only if overwrite is intended.
- No checks run: verify `[[check]]` entries and any `--only/--group` filters.
- Daemon not running: check `healthd daemon status` and `healthd daemon logs --lines 100`.
- Alerts not sent: verify notifier blocks and run `healthd notify test --backend <name>`.
- Command checks flaky: increase `timeout`, run command manually, then retest with `healthd check`.

## Rollback hints

- Keep a backup before risky edits: `cp <path> <path>.bak`.
- To rollback config quickly: `cp <path>.bak <path>`, then `healthd validate --config <path>`.
- If daemon behavior regresses, uninstall first, then restore previous scheduler only after confirmation.

## References

- Config snippets: `references/config-examples.md`
- OpenClaw operator flow: `references/openclaw-integration.md`
