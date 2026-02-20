# AGENTS.md

Project instructions for coding agents in `healthd`.

## Stack
- Language: Go
- CLI: Cobra
- Config: TOML (strict parsing + validation)

## Workflow
- Build in small vertical slices aligned to issues.
- Prefer stacked PRs with Graphite for dependent work.
- Keep behavior deterministic; avoid hidden magic.

## Quality Gate (must pass before commit)

```bash
go fmt ./...
go test ./...
```

When linters are added, include them in the gate.

## Conventions
- Keep packages focused and testable.
- Parse config into typed structs; reject unknown keys.
- No auto-remediation in v1 (detect/report only).
- Do not add unrelated changes in the same branch.
