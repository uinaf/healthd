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
