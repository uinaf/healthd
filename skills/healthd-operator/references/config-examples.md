# healthd config examples

## Minimal baseline (single local check + webhook)

```toml
interval = "60s"
timeout = "10s"

[[check]]
name = "disk-root"
group = "host"
command = "df -P / | awk 'NR==2 {print 100-$5}'"

[check.expect]
min = 10

[notify]
cooldown = "5m"

[[notify.backend]]
name = "ops-webhook"
type = "webhook"
url = "https://example.internal/healthd-alerts"
```

## Multi-check host profile (disk + load + HTTP + ntfy)

```toml
interval = "60s"
timeout = "10s"

[[check]]
name = "disk-root"
group = "host"
command = "df -P / | awk 'NR==2 {print 100-$5}'"

[check.expect]
min = 10

[[check]]
name = "load-1m"
group = "host"
command = "uptime | awk -F'load averages?: ' '{print $2}' | awk -F', ' '{print $1}'"

[check.expect]
max = 8

[[check]]
name = "api-health"
group = "service"
command = "curl -fsS --max-time 5 http://127.0.0.1:3000/healthz >/dev/null"

[check.expect]
exit_code = 0

[notify]
cooldown = "5m"

[[notify.backend]]
name = "pager-ntfy"
type = "ntfy"
topic = "ops/healthd"

[[notify.backend]]
name = "local-hook"
type = "command"
command = "./scripts/on-healthd-alert.sh"
timeout = "5s"
```

## Useful command patterns

```bash
# Validate config before daemon operations
healthd validate --config ~/.config/healthd/config.toml

# Run one check only (name filter)
healthd check --config ~/.config/healthd/config.toml --only disk-root

# Run one group only
healthd check --config ~/.config/healthd/config.toml --group service

# JSON output for automation
healthd check --config ~/.config/healthd/config.toml --json

# Test only ntfy backend
healthd notify test --config ~/.config/healthd/config.toml --backend ntfy
```

## Validation gotchas

- Unknown keys are rejected (strict TOML decoding).
- `interval`, `timeout`, and `notify.cooldown` must be positive durations.
- Each notifier needs type-specific fields:
  - `webhook` → `url`
  - `ntfy` → `topic`
  - `command` → `command`
- Backend names (or fallback type names) must be unique.
