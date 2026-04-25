---
name: healthd-operator
description: Operate healthd safely on a local host: detect environment, install/build binary, bootstrap config, validate and run one-shot checks, test notifications, inspect alert history. Use when asked to deploy healthd, troubleshoot health checks, migrate from cron, or verify alert delivery. Daemon lifecycle (start/stop/restart) is owned by an external supervisor (process-compose, systemd, launchd) — this skill drives healthd directly, not the supervisor.
homepage: https://github.com/uinaf/healthd
metadata:
  {
    "openclaw":
      {
        "emoji": "🏥",
        "requires": { "bins": ["healthd"] },
        "install":
          [
            {
              "id": "brew",
              "kind": "brew",
              "formula": "uinaf/tap/healthd",
              "bins": ["healthd"],
              "label": "Install healthd (brew)",
            },
          ],
      },
  }
---

# healthd Operator

Use this skill for practical host operations. Prefer reversible steps and show commands before running them.

## Safety rules

- Do **not** overwrite configs or remove cron jobs without explicit confirmation.
- Before changing supervised daemon state, capture current context (config path, supervisor status, existing cron entries).
- Keep edits scoped to requested host/application checks.

## Workflow

1. **Detect environment/context**
   - Confirm OS/user, repo vs installed binary, config path (`--config` / `HEALTHD_CONFIG` / default).
   - Check whether a supervisor (process-compose / systemd / launchd) is running healthd; do not assume.
2. **Install/build binary**
   - Preferred install: `brew install uinaf/tap/healthd`
   - Direct installer alternative: `curl -fsSL https://raw.githubusercontent.com/uinaf/healthd/main/scripts/install.sh | bash`
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
6. **Run continuously**
   - Foreground (debugging): `healthd run --config <path>` — Ctrl-C to stop.
   - Production: configure your supervisor (process-compose, systemd unit, launchd plist) to invoke the same command. Lifecycle (start/stop/restart/logs) is the supervisor's responsibility, not healthd's.
7. **Inspect history**
   - Recent transitions: `tail -n 20 ~/.local/state/healthd/alerts.log`
   - Live dashboard: `healthd status --watch --config <path>`

## Troubleshooting checklist

- Config parse/validation fails: run `healthd validate --config <path>` and fix unknown keys/types.
- `healthd init` fails with existing file: rerun with `--force` only if overwrite is intended.
- No checks run: verify `[[check]]` entries and any `--only/--group` filters.
- Daemon not running: check via your supervisor (e.g. `process-compose process list`, `launchctl list | grep healthd`, `systemctl --user status healthd`).
- Command checks failing with `exit 127` under a supervisor: launchd/systemd default `PATH` is minimal. Set an explicit `PATH` in the supervisor config so command checks can find Homebrew/system binaries.
- Alerts not sent: verify notifier blocks and run `healthd notify test --backend <name>`.
- Alerts panel empty in `healthd status`: alerts only appear after a transition (fail or recover). Confirm `~/.local/state/healthd/alerts.log` is being written by the running daemon.
- Command checks flaky: increase `timeout`, run command manually, then retest with `healthd check`.

## Rollback hints

- Keep a backup before risky edits: `cp <path> <path>.bak`.
- To rollback config quickly: `cp <path>.bak <path>`, then `healthd validate --config <path>`.
- If supervised daemon behavior regresses, ask the supervisor to stop healthd, restore prior config or scheduler, and only then restart — confirm exact commands before running.

## References

- Config snippets: `references/config-examples.md`
- OpenClaw operator flow: `references/openclaw-integration.md`
