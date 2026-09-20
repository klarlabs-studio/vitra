# Vitra — Project Intent

> **Status:** Working intent / implementation charter  
> **Working name:** Vitra  
> **Language:** Go  
> **Scope:** Secure, extensible, cross-platform application runtime for
> building desktop applications with Go and web frontends.  
> **Date:** 2026-09-06

This document is the product charter. Implementation follows DDD/TDD under
Klarlabs conventions. Phase 1 (this repository bootstrap) delivers the
**secure runtime kernel**: capability gateway, explicit commands, caller
identity, inspectable privileged surface, and CLI skeletons.

## Why Vitra Exists

Vitra exists to make building a high-quality native desktop application
with Go and web technology feel like building a well-designed Go
service: explicit, composable, strongly typed, secure by default,
observable, testable, and unsurprising.

The project is not justified by the fact that Go lacks a WebView
wrapper. It does not. Wails is a capable and rapidly improving
framework, and native WebView bindings already exist at lower levels.
Vitra is justified only if it can establish a materially better
application-runtime model: a small trusted Go core, a deliberately
untrusted frontend, capability-oriented native access, first-class
process isolation, a stable plugin contract, reproducible distribution,
and excellent developer tooling.

## Product Thesis

Modern desktop development has an attractive but dangerous seam: web UI
gives teams an exceptional rendering and component ecosystem, while
native application code provides filesystem, process, network, device,
credential, and operating-system access.

Frameworks become risky or difficult when they make that seam invisible.

**Vitra will make the seam a first-class architectural boundary.**

```text
┌──────────────────────────────────────────────────────────┐
│                    Web Frontend                          │
│          Vue / React / Svelte / vanilla / etc.           │
│                   UNTRUSTED ZONE                         │
└─────────────────────────┬────────────────────────────────┘
                          │ typed messages
                          ▼
┌──────────────────────────────────────────────────────────┐
│                  Capability Gateway                      │
│ origin • window • permission • scope • schema • limits   │
└─────────────────────────┬────────────────────────────────┘
                          │ authorized invocation
                          ▼
┌──────────────────────────────────────────────────────────┐
│                   Vitra Runtime                          │
│ lifecycle • windows • events • plugins • supervision     │
│                     TRUSTED ZONE                         │
└──────────────┬──────────────────────────┬────────────────┘
               │                          │
               ▼                          ▼
       Native OS adapters          Isolated workers
```

## Non-Goals

Vitra is not intended to build a custom rendering engine, bundle
Chromium by default, expose arbitrary Go objects through reflection,
treat IPC as an implementation detail, make insecure configurations the
shortest path, require a Node backend, become a widget toolkit, or
pursue mobile until the desktop security model is excellent.

## Design Principles (summary)

1. Secure by construction  
2. Least privilege is the default  
3. Intent over sanitization  
4. Strong typing end to end  
5. Explicit platform semantics  
6. Small trusted computing base  
7. Failure containment  
8. Observable runtime  
9. Distribution is part of the framework  
10. Go-native composition  

## Security Invariants (architectural tests)

These are encoded as tests in `domain/` and `vitra_test.go` where the
kernel can enforce them today:

1. A newly created WebView has no privileged native API access unless a capability grants it.
2. Navigation to a new origin cannot accidentally retain authority granted to another origin.
3. Frontend input is never trusted solely because it came through generated TypeScript.
4. IPC sender identity is validated at the native boundary.
5. Every privileged frontend-callable operation maps to an inspectable permission.
6. A plugin cannot silently expand another plugin’s permission scope.
7. Production configuration does not expose generic arbitrary shell execution by default.
8. The WebView host does not require administrator/root privileges.
9. Update artifacts are never installed without integrity/authenticity verification.
10. Secrets used for signing are not required to reside in the project configuration.
11. Development-only privileges cannot silently leak into release builds.
12. Remote/untrusted content has no native authority by default.
13. Capability denial is deterministic and observable.
14. Unsupported platform behavior is surfaced explicitly.
15. Resource and subscription ownership is cleaned up when its window/WebView disappears.

## Phased Implementation

| Phase | Focus |
|-------|--------|
| 0 | Architecture spikes (WebView + IPC per OS) — see `docs/spikes/phase0.md` |
| 1 | Secure runtime kernel |
| 2 | Desktop completeness — see `docs/spikes/phase2.md` |
| 3 | Plugin SDK and ecosystem — see `docs/spikes/phase3.md` |
| **4** | **Distribution excellence** — see `docs/spikes/phase4.md` |
| 5 | Isolation and enterprise hardening |

## Decision Rule

Stop or redirect Vitra if implementation spikes show that its
differentiated properties require reproducing large amounts of
Wails/Tauri work without a materially better security, reliability,
extensibility, or developer experience. Contributing missing
capabilities to Wails remains a valid outcome.
