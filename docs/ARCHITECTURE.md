# Architecture

Local host health-check daemon. Runs checks, tracks state, sends alerts on transitions.

## System Context

```mermaid
graph LR
  PC[process-compose] -->|invokes healthd run| Loop
  Loop -->|shell exec| Checks[Health Checks]
  Loop -->|on transition| Notify[Notifiers]
  Loop -->|on transition| Alerts[(alerts.log)]
  Notify --> Ntfy[(ntfy)]
  Notify --> Cmd[(command)]
  CLI[CLI] -->|one-shot| Checks
  CLI -->|TUI| Status[Status View]
  Status --> Alerts
  Loop & CLI -->|read| Config[(~/.config/config.toml)]
```

## Components

| Component | Responsibility |
|-----------|---------------|
| **cmd** | Cobra CLI: check, status, run, init, validate, notify |
| **runner** | Execute checks, filter, collect results |
| **loop** | Continuous run loop, fail/recover transition tracking |
| **notify** | Alert backends (ntfy, command), cooldown logic |
| **alertlog** | Append-only writer for `~/.local/state/healthd/alerts.log` (read by TUI) |
| **tui** | Bubbletea status display, watch mode |
| **config** | TOML parsing, validation |

## Check Lifecycle

```mermaid
stateDiagram-v2
  [*] --> Passing: check passes
  [*] --> Failing: check fails
  Passing --> Failing: check fails → alert CRIT
  Failing --> Passing: check passes → alert RECOVERED
  Failing --> Failing: still failing (cooldown suppresses)
```

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Shell-exec checks | Run any command | Maximum flexibility, no custom check API |
| Detect only (v1) | No auto-remediation | Keep it simple, alert humans |
| Transition alerts | Only on state change | No alert spam |
| Cooldown per backend | Configurable per notifier | Different urgency per channel |
| Status TUI bypasses bubbletea in non-watch | Direct View() render | Works without TTY (CI, pipes) |
