# healthd Rollout: Migration from cron

This rollout migrates from `glitch-health-check` cron jobs to `healthd` daemon mode.

## 1. Prepare config

1. Copy `examples/current-host.toml` to your local config path.
2. Adjust thresholds and service endpoints for this host.
3. Validate config:

```bash
healthd validate --config ~/.config/healthd/config.toml
```

## 2. Shadow mode (48h)

Run `healthd` in parallel with existing cron checks for 48 hours.

1. Install daemon while keeping cron active:

```bash
healthd daemon install --config ~/.config/healthd/config.toml
```

2. Verify daemon is active:

```bash
healthd daemon status
```

3. Compare outputs at least every 12 hours:

```bash
healthd check --config ~/.config/healthd/config.toml --json
```

4. Collect alert stats and compare with existing cron alerts.

## 3. Success metrics (must be explicit before cutover)

- No missing critical incidents versus cron during the 48h shadow window.
- False positive rate is equal to or lower than cron alerts.
- `healthd daemon status` reports running for the full 48h.
- `healthd notify test` succeeds for all production backends.

## 4. Cutover checklist

Complete all items before disabling cron:

- [ ] 48h shadow mode completed
- [ ] Success metrics met
- [ ] On-call acknowledges alert quality is acceptable
- [ ] `healthd daemon status` shows running with a PID
- [ ] Rollback owner assigned

Disable the cron job after checklist completion:

```bash
crontab -l | grep -v 'glitch-health-check' | crontab -
```

## 5. Post-cutover checks

- Run `healthd check --config ~/.config/healthd/config.toml` once manually.
- Confirm latest daemon logs are clean:

```bash
healthd daemon logs --lines 100
```
