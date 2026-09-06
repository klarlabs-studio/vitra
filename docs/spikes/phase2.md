# Phase 2 — Desktop Completeness

Phase 2 makes Vitra usable as a multi-window desktop shell while preserving
Phase 1’s capability boundary.

## Delivered

| Area | Package / API |
|------|----------------|
| Window-owned events | `domain.Subscription`, `SubscribeEventUseCase` |
| Subscription cleanup on close | `CloseWindowUseCase` releases subscriptions |
| Navigation policy | `domain.NavigationPolicy`, `NavigateWithPolicyUseCase` |
| Deep-link patterns | `domain.DeepLinkPattern` |
| Capability-gated desktop services | `desktop` — menu, tray, dialog, clipboard, shortcuts, single-instance, deeplinks |
| Explicit unsupported features | services call `platform.Require` before native hooks |

## Security / reliability invariants covered

- **2 / 12**: untrusted remote origins denied by default navigation policy
- **5**: every desktop surface maps to an inspectable permission (`menu.set`, `clipboard.read`, …)
- **14**: missing platform features → `platform.ErrUnsupported`, never silent success
- **15 / reliability 2**: closing a window closes and deletes its subscriptions

## Still platform-adapter work (later)

Native macOS/Windows/Linux bindings for WebView chrome, menus, tray, and
dialogs remain adapter implementations behind `platform.Host`. Phase 2
freezes the portable contracts and enforces grants in front of them.
