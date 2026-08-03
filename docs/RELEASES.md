# Releases

Pushes to `main` release automatically. Skip a push with `[skip ci]`.

## Versioning

Conventional Commits drive the bump (see `.releaserc.json`):

| Commit type | Release |
|---|---|
| `feat:` | minor |
| `fix:` / `perf:` / `refactor:` | patch |
| `feat!:` / breaking change | major |
| `docs:` / `test:` / `chore:` / `build:` / `ci:` | none |

## Pipeline

1. `verify` runs with read-only credentials
2. Protected `release` Environment mints a short-lived `uinaf-releaser` installation token scoped to `healthd` + `homebrew-tap`
3. `semantic-release` creates the GitHub Release
4. GoReleaser publishes darwin/arm64 + darwin/amd64 archives and updates `uinaf/homebrew-tap`

Sources of truth: `.github/workflows/ci.yml`, `.releaserc.json`, `.goreleaser.yaml`.

## Credentials

`release` Environment:

| Name | Kind |
|---|---|
| `UINAF_RELEASE_APP_ID` | variable |
| `UINAF_RELEASE_APP_PRIVATE_KEY` | secret |
