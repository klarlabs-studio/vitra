# Phase 0 — Architecture Spikes

> Exit criterion: enough evidence to freeze the first runtime protocol.

This phase proves the seams that Phase 1’s kernel assumes, without yet
shipping production WebView adapters.

## Spikes completed (in-repo)

| Spike | Evidence |
|-------|----------|
| Packaged frontend messaging contract | `ipc` package — versioned envelope, invoke/result/error kinds |
| JS→native / native→JS message shape | `ipc.Bridge`, `EncodeResult`, `EncodeError` |
| Caller / origin / window identity at boundary | `Bridge.DecodeInvoke` discards payload-claimed identity; tests encode invariants 3 & 4 |
| Capability evaluation (kernel) | Phase 1 `domain.CapabilityGateway` (already landed) |
| Platform feature discovery | `platform.FeatureSet` + `platform.Require` |
| Unsupported → explicit error | `platform/null` returns `ErrUnsupported` for dialog/menu/tray/clipboard |
| Minimal window + message on stub host | `null.Host` create/navigate/post/close |

## Platform notes (freeze inputs for adapters)

### macOS (WKWebView)

- Content process is separate from the app process; IPC must treat the
  renderer as untrusted.
- Navigation changes origin; authority must not follow (invariant 2).
- Hardened Runtime / App Sandbox constrain FS and network — capability
  grants should align with entitlements rather than fight them.

### Windows (WebView2)

- Evergreen WebView2 Runtime is a prerequisite — `vitra doctor` must
  diagnose missing/outdated runtimes (Phase 2+).
- WebView2 can run out-of-process; host must stamp window identity on
  every message, never accept it from script.

### Linux (WebKitGTK)

- GTK3/GTK4 and WebKitGTK version skew is the dominant compatibility
  risk; feature matrix must report versions and known incompatibilities.
- Wayland vs X11 affects dialogs, tray, and global shortcuts — expose as
  feature support, not silent no-ops.

## Frozen protocol (v1)

```json
{
  "protocol": "1",
  "kind": "invoke",
  "id": "corr-id",
  "payload": {
    "command": "project.open",
    "input": {},
    "resource_path": "/optional/path"
  }
}
```

Rules:

1. `protocol` must equal `ipc.ProtocolVersion`.
2. Host adapter injects `Caller{Window, Origin}` — payload `window` /
   `origin` fields are ignored if present.
3. Results use `kind=invoke_result`; denials/errors use `kind=error`
   with stable `code` values from `domain.DenialCode`.

## Out of scope for Phase 0

- Production CGO / syscall WebView bindings (Phase 2 adapters)
- Codegen of TypeScript bindings (Phase 3)
- Packaging / signing (Phase 4)
- Supervised workers (Phase 5)

## Decision

Proceed to Phase 1 kernel (already merged on this lineage) and Phase 2
desktop completeness. Differentiating properties (capability gateway,
identity-at-boundary IPC, explicit unsupported) do not require
reproducing Wails/Tauri wholesale.
