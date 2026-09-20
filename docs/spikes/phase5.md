# Phase 5 — Isolation & Enterprise Hardening

Phase 5 adds failure containment and fleet-friendly controls without growing
the WebView host’s privilege.

## Delivered

| Area | Package |
|------|---------|
| Supervised workers | `worker.Supervisor` |
| Crash-safe bookkeeping | worker records survive runner failures |
| Audit log sink | `audit.MemorySink` (+ `Sink` port) |
| Enterprise policy overlay | `policy.Engine` |
| Live Runtime wiring | `Runtime.SetPolicy` / `Authorize` + invocation overlay |

## Runtime wiring

`Runtime.SetPolicy` installs a `policy.Engine` that can only tighten decisions:

- `Authorize` applies `OverlayDecision` after the capability gateway
- `Invoke` applies the same overlay via `InvocationService.Overlay`
- `ApplyUpdate` calls `AuthorizeUpdate` before signed install (channel + unsigned reject)

Production engines force signed updates and strip development privileges
(invariants 9 / 11). The competitive demo honors optional env hooks:

```bash
VITRA_POLICY=production
VITRA_POLICY_DENY=shell.exec,clipboard.read
```

## Invariants

- **8**: elevated work is modeled as `Spec.Elevated` workers — the host stays non-admin
- **9 / 11**: production policy forces signed updates and strips development privileges
- **Reliability 4**: worker crashes update `Record` state; they do not wipe supervisor maps
- Audit events cover capability decisions, plugin registration, worker lifecycle, updates

## Non-goals in this PR

OS process spawning / IPC transport for workers (adapters), SIEM exporters,
and MDM-specific policy document formats — those plug into these ports.
