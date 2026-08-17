# Contributing

## Setup

Go is pinned in `.tool-versions` / `go.mod` (currently 1.26).

```bash
git config core.hooksPath .git-hooks
go build ./...
```

## Validation

Run the full gate before opening or updating a pull request:

```bash
go run ./cmd/verify
```

That runs fmt, golangci-lint, tests, and coverage gates (80% total, 70% per package with statements).

Fast iteration:

```bash
go test ./...
go test ./integration/... -v
go test ./e2e/cli/... -v
```

## Pull Requests

- Keep changes focused on one concern
- Use Conventional Commits (`feat:`, `fix:`, `docs:`, …); they drive releases
- Fill the pull request template and include validation evidence
- Update docs when behavior or commands change
- Keep vulnerability reports out of public issues; use [Security](https://github.com/uinaf/healthd/security/policy)

## Releases

Successful pushes to protected `main` evaluate Conventional Commits after
`verify` passes. `fix`, `perf`, and `refactor` publish patches; `feat`
publishes a minor; breaking changes publish a major; and docs, test, chore,
build, and CI changes do not publish.

The release job mints a short-lived `uinaf-releaser` token inside the
`release` Environment, creates the tag and GitHub Release, and updates the
Homebrew tap. See [Releases](docs/RELEASES.md) for the complete contract.
