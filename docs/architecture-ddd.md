# Vitra DDD Architecture

This document describes the bounded contexts, ubiquitous language, and
layering rules that govern the Vitra codebase. Every Go package belongs
to exactly one layer and obeys the dependency direction below.

**Allowed import direction**: adapters → application → domain.

```
vitra (root)     Fluent runtime facade — what consumers import.
domain/          Aggregates, value objects, domain services, ports. Zero deps.
application/     Use cases that orchestrate the domain.
inmemory/        In-memory port adapters (tests + single-process kernel).
cmd/vitra/       Developer CLI (delivery adapter).
example/         Runnable documentation — not a supported API.
```

Infrastructure (platform WebView adapters, packaging, updater) will land
under dedicated packages in later phases and may import application/domain
only. Nothing in `domain/` may import an infrastructure package.

## Bounded Contexts (Phase 1)

| Context | Package | Core concepts |
|---------|---------|---------------|
| Identity & values | `domain` | `AppID`, `WindowID`, `Origin`, `PermissionName`, `CommandName`, `GrantName` |
| Window lifecycle | `domain` | `Window` aggregate, navigate/close |
| Capability | `domain` | `CapabilityGrant`, `PathScope`, `CapabilityGateway`, `Decision` |
| Commands | `domain` | `CommandDefinition`, `InvocationService` |
| Resources | `domain` | `ResourceHandle` ownership |
| Runtime orchestration | `application` | open/navigate/close window, register grant/command, invoke, inspect |
| Delivery | `vitra`, `cmd/vitra` | facade + CLI |

## Ubiquitous Language

- **Caller** — validated IPC sender identity (`Window` + `Origin`) established at the native boundary, never from message payload fields alone.
- **Capability grant** — explicit, inspectable authority binding windows, origins, and permissions (with optional path scope).
- **Command** — explicitly registered application operation; exported Go methods are not commands.
- **Permission** — named privileged native operation (e.g. `fs.read`).
- **Origin** — content origin of a WebView; packaged apps use `app://local`.
- **Denial** — deterministic, coded capability refusal (`ErrDenied` + `DenialCode`).
- **Effective surface** — inspectable projection of privileges for a window at its current origin.

## Aggregates

| Aggregate | Invariants |
|-----------|------------|
| `Window` | Non-empty id/origin; navigation forbidden when closed; caller unavailable when closed |
| `CapabilityGrant` | At least one window, origin, and permission; no duplicate permissions; deny paths win |
| `CommandDefinition` | Non-empty name and required permission |
| `ResourceHandle` | Owned by exactly one window; close is idempotent |

## Domain Services

- **`CapabilityGateway`** — evaluates grants; first allow wins; denials are specific and stable.
- **`InvocationService`** — identify → resolve command → authorize → execute.

## Ports (owned by domain)

- `GrantRepository`, `WindowRepository`, `CommandRepository`, `ResourceRepository`
- `CommandExecutor`, `CommandExecutorLookup`

## Composition Root

`vitra.New(Config)` is the composition root. It wires in-memory adapters and
use cases. Platform hosts (future) should construct the same use cases with
platform-backed ports rather than inventing parallel invocation paths.

## Testing

- Domain tests use in-package fakes or pure constructors (no `inmemory` import required; some tests use local fakes).
- Application tests drive use cases with `inmemory/` adapters.
- Root `vitra_test` covers security invariants end-to-end through the facade.
- Prefer table-driven tests; race-detector clean for concurrent repositories.

## Security Mapping

Architectural tests live next to the code that enforces them. See
`docs/intent.md` § Security Invariants and `domain/gateway_test.go`,
`domain/invocation_test.go`, `vitra_test.go`.
