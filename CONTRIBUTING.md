# Contributing to vitra

Thanks for considering a contribution. Vitra aims to stay small, principled,
and secure-by-construction — please keep that in mind when proposing changes.

## Core principles

1. **Zero external dependencies in the kernel.** The `domain/` package must not
   import anything outside the standard library. Phase 1 adapters (`inmemory/`)
   also use stdlib only. Platform WebView bindings (later phases) are adapters.
2. **DDD boundaries.** Respect the dependency direction:
   `domain` ← `application` ← `inmemory` ← `vitra` (root facade) ← consumer code.
   The domain owns its port interfaces.
3. **Least privilege by default.** New windows have no ambient authority.
   Commands are explicitly registered. Grants must name windows, origins, and
   permissions.
4. **Aggregates enforce invariants.** Unexported fields, constructor validation,
   defensive copies.
5. **TDD.** Start with failing tests — especially for security invariants.

## Development setup

```bash
git clone https://github.com/klarlabs-studio/vitra.git
cd vitra
make install-hooks
make check
```

You need Go 1.26+, `golangci-lint` v2, and optionally `nox` + `coverctl`.

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/) and sign off
with `git commit -s` (DCO).

## Pull request checklist

- [ ] `make check` passes locally
- [ ] Tests added for new behavior (TDD preferred)
- [ ] Race detector clean: `go test ./... -race`
- [ ] Security invariants still hold
- [ ] Docs updated if public API or architecture changed
