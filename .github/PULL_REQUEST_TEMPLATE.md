## What and why

<!-- The problem, and how this change solves it. Link issues with "Fixes #123". -->

## Security impact

<!-- Does this add or change privileged behavior (commands, grants, path handling, navigation,
IPC, native host calls, updates)? If so, which invariant protects it and which test proves it? -->

## Checklist

- [ ] `make check` passes (fmt, lint, test, security)
- [ ] Tests added for new behavior; security invariants encoded as tests
- [ ] `go test ./... -race` clean
- [ ] `CHANGELOG.md` updated under Unreleased
- [ ] Docs updated if public API or behavior changed
- [ ] Commits signed off (`git commit -s`, DCO)
