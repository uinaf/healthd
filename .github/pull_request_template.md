## Summary

<!-- 1-3 sentences. Why this change exists. -->

## Changes

<!-- Bullet list of what changed at a high level. -->

## Validation

<!-- How did you prove this works? Tick what applies and add evidence. -->

- [ ] `go run ./cmd/verify` passes locally (fmt + lint + test + coverage gate)
- [ ] Pre-push hook is active (`git config core.hooksPath .git-hooks`)
- [ ] New behavior covered by tests (unit, integration, or e2e)
- [ ] Manual smoke against `healthd check` / `healthd run` / `healthd status` if user-facing
- [ ] Docs updated (AGENTS.md, README.md, ARCHITECTURE.md, operator skill) if behavior or commands changed

## Risks

<!-- What could break? Anything reviewers should look at extra carefully? -->

## Linked Issues

<!-- Closes #N, refs #N -->
