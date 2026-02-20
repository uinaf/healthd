# healthd

Pluggable host health-check daemon written in Go.

## Goal

Replace cron-based health checks with a local daemon that:

- runs command-based checks on schedule
- emits stable JSON output (`--json`) for automation
- sends transition-based alerts (`ok/warn/crit/recovered`)
- supports one-shot and daemon modes

## Planned Commands

- `healthd check`
- `healthd check --json`
- `healthd validate`
- `healthd notify test`
- `healthd daemon install|uninstall|status|logs`

## Status

Bootstrapped repository + issue plan created.
Implementation starts with config + validation and proceeds as stacked PRs.
