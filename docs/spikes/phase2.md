# Phase 2 — Desktop Completeness

Phase 2 makes Vitra usable as a multi-window desktop shell while preserving
Phase 1’s capability boundary.

## Delivered

| Area | Package / API |
|------|----------------|
| Window-owned events | `domain.Subscription`, `SubscribeEventUseCase`, `EmitEventUseCase` |
| Host→frontend delivery | `Runtime.EmitEvent`, `app.App.Emit`, `bridge` `vitra.on` |
| Subscription cleanup on close | `CloseWindowUseCase` releases subscriptions |
| Navigation policy | `domain.NavigationPolicy`, `NavigateWithPolicyUseCase` |
| Deep-link patterns | `domain.DeepLinkPattern` |
| Capability-gated desktop services | `desktop` — menu (+ in-window accelerators), tray, dialog, clipboard, shortcuts, single-instance, deeplinks, drag-drop, window chrome, open URL |
| Explicit unsupported features | services call `platform.Require` before native hooks |
| Linux xdg MIME associations | `linux.RegisterFileAssociations`, `vitra register-files` |

## Security / reliability invariants covered

- **2 / 12**: untrusted remote origins denied by default navigation policy
- **5**: every desktop surface maps to an inspectable permission (`menu.set`, `clipboard.read`, …)
- **14**: missing platform features → `platform.ErrUnsupported`, never silent success
- **15 / reliability 2**: closing a window closes and deletes its subscriptions

## Platform adapters (landed in competitive / DesktopHost track)

Phase 2 froze portable contracts (`desktop.*`, `platform.Require`). Native
bindings now ship under `-tags vitra_native` on Linux (WebKitGTK), Darwin
(WKWebView), and Windows (Win32 + WebView2) — see
[`docs/spikes/competitive.md`](competitive.md). Wayland global hotkeys stay
`ErrUnsupported` (use in-window `MenuItem.Shortcut`).
