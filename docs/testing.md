# Testing Matrix

## Required (CI default)

- Unit and package tests: `go test ./...`
- Integration CLI coverage: `go test ./integration/... -v`
- CLI E2E-lite with built binary: `go test ./e2e/cli/... -v`

These tests are deterministic and do not require network access.

## Optional (host-level macOS)

- Host daemon scaffold (self-hosted macOS only):
  - `HEALTHD_HOST_E2E=1 go test -tags=hoste2e ./e2e/host/... -v`
  - For install/uninstall lifecycle checks, also set `HEALTHD_HOST_E2E_ALLOW_INSTALL=1`.

This suite is intentionally excluded from default CI and only runs in the optional self-hosted workflow.

## Local Commands

```bash
go test ./...
go test ./integration/... -v
go test ./e2e/cli/... -v
```
