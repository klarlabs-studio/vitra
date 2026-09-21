# Phase 5 — Isolation & Enterprise Hardening

Phase 5 adds failure containment and fleet-friendly controls without growing
the WebView host’s privilege.

## Delivered

| Area | Package |
|------|---------|
| Supervised workers | `worker.Supervisor` + `CommandRunner` + `StdioIPC` + `Runtime.StartWorker` |
| Crash-safe bookkeeping | worker records survive runner failures |
| Audit log sink | `audit.MemorySink` (+ `Sink` port) |
| SIEM exporters | `audit.JSONLSink` (NDJSON) + `audit.CEFSink` (Common Event Format) + `MultiSink` |
| MDM policy documents | `policy.LoadDocument` / `Document.Save` (JSON; unknown fields rejected) |
| Enterprise policy overlay | `policy.Engine` |
| Live Runtime wiring | `Runtime.SetPolicy` / `SetAudit` / workers on Authorize, Invoke, plugins, updates |

## Runtime wiring

`Runtime.SetPolicy` installs a `policy.Engine` that can only tighten decisions:

- `Authorize` applies `OverlayDecision` after the capability gateway
- `Invoke` applies the same overlay via `InvocationService.Overlay`
- `ApplyUpdate` calls `AuthorizeUpdate` before signed install (channel + unsigned reject)

`Runtime.SetAudit` installs an `audit.Sink`. Live paths emit:

| Kind | Source |
|------|--------|
| `capability.decision` | `Authorize` |
| `policy.override` | policy tighten on `Authorize` |
| `command.invoke` | `Invoke` |
| `plugin.register` | `RegisterPlugin` |
| `update.plan` | `ApplyUpdate` |
| `worker.lifecycle` | `StartWorker` / `StopWorker` / crash·stop transitions |

SIEM exporters implement the same `Sink` port:

- `JSONLSink` — one JSON object per line for file/pipe agents
- `CEFSink` — ArcSight-compatible CEF lines (`FormatCEF` for custom writers)
- `MultiSink` — fan-out (e.g. memory + JSONL)

MDM/fleet policy documents are JSON `policy.Document` values (`LoadDocument` /
`Save` / `ParseDocument`). `Engine.Document()` returns the effective document
(including production-forced signature and dev-privilege flags).

`Runtime.StartWorker` supervises in-process runners and OS processes
(`worker.CommandRunner`: direct exec, context cancel stops the process).
Host↔worker messaging uses `worker.Session` (JSON lines) via `PipePair` or
`StdioIPC` on the child's stdin/stdout. Elevated work stays off the WebView
host (invariant 8).

Production engines force signed updates and strip development privileges
(invariants 9 / 11). The competitive demo honors optional env hooks:

```bash
VITRA_POLICY=production
VITRA_POLICY_FILE=/etc/vitra/fleet.json   # optional MDM JSON document
VITRA_POLICY_DENY=shell.exec,clipboard.read
VITRA_AUDIT=1                             # memory
VITRA_AUDIT=jsonl VITRA_AUDIT_PATH=audit.ndjson
VITRA_AUDIT=cef   VITRA_AUDIT_PATH=audit.cef
```

## Invariants

- **8**: elevated work is modeled as `Spec.Elevated` workers — the host stays non-admin
- **9 / 11**: production policy forces signed updates and strips development privileges
- **Reliability 4**: worker crashes update `Record` state; they do not wipe supervisor maps
- Audit events cover capability decisions, plugin registration, worker lifecycle, updates

## Non-goals

Vendor-specific SIEM connectors (Splunk HEC, Sentinel, etc.) and proprietary MDM
wire protocols — JSONL/CEF and JSON policy documents are the portable ports those
systems plug into. OS processes are supervised via `worker.CommandRunner` /
`StdioIPC` (direct exec, no shell).
