# CLAUDE.md

Guidance for Claude Code (and other agents) working in this repository.

## What Vitra Is

A secure, capability-oriented desktop application runtime for Go + web
frontends. Phase 1 is the **secure runtime kernel** — capability gateway,
explicit commands, caller identity, inspectable surface. It is a library you
embed; platform WebView adapters and packaging come later.

Read `docs/intent.md` and `docs/architecture-ddd.md` before substantive changes.

## Build & Test

```bash
make check          # fmt + lint + test + security
make test
make lint
make cover
go test ./... -race
go run ./example/quickstart
go run ./cmd/vitra -- help
```

Zero external dependencies in the kernel — standard library only.

## Architecture

```
vitra (root)     Fluent Runtime facade
domain/          Aggregates, services, ports (zero deps)
application/     Use cases
inmemory/        Default adapters
cmd/vitra/       CLI delivery adapter
example/         Runnable docs
```

Dependency direction: `domain` ← `application` ← `inmemory` ← `vitra`.

## Key Design Rules

- Domain has zero imports outside stdlib.
- New windows have no privileged access without an explicit grant.
- Navigation changes origin; authority does not follow.
- Caller identity is established at the native boundary (window repo), not from payload fields.
- Path scopes reject `..` traversal; deny patterns win over allow.
- Capability denials are deterministic (`DenialCode` + reason).
- Do not add ambient “allow all windows/origins” grant APIs.
- Do not auto-export Go methods as frontend commands.

## Security Invariants

Encode new privileged behavior as tests. See `docs/intent.md` §9 and existing
tests in `domain/gateway_test.go`, `domain/invocation_test.go`, `vitra_test.go`.
