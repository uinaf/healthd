# healthd Operational Runbook

## Validation

```bash
healthd validate --config ~/.config/healthd/config.toml
```

Expected: `config is valid` message.

## One-shot check

```bash
healthd check --config ~/.config/healthd/config.toml
```

Expected: grouped PASS/FAIL output and summary.

## Daemon status

```bash
healthd daemon status
```

Expected: `status: running (pid <n>)` after installation.

## Daemon PATH notes (macOS launchd)

`launchd` uses a limited default PATH (`/usr/bin:/bin:/usr/sbin:/sbin`).
As of current releases, `healthd daemon install` writes an explicit PATH into the LaunchAgent plist:

`/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`

This prevents command checks from failing with `exit 127` when binaries live under Homebrew paths.

If you still see command-not-found failures, inspect daemon logs:

```bash
healthd daemon logs --lines 100
```

## Rollback

Use rollback if alert quality regresses or daemon repeatedly exits.

1. Stop/remove LaunchAgent:

```bash
healthd daemon uninstall
```

2. Re-enable previous cron checks:

```bash
( crontab -l; echo "*/5 * * * * /usr/local/bin/glitch-health-check" ) | crontab -
```

3. Verify cron entry exists and health checks resume.
