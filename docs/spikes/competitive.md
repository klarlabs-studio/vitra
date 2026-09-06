# Competitive desktop runtime

This spike turns Vitra from a kernel-only stack into a **runnable** secure
desktop runtime on Linux (WebKitGTK), with API-compatible Darwin/Windows hosts
and a developer CLI that matches Wails-class DX: `new` / `dev` / `build` /
`doctor`.

## What ships

| Surface | Status |
|---------|--------|
| `app.App` — assets + WebView + invoke→gateway | Done |
| `bridge.PreloadJS` — `window.vitra.invoke` | Done |
| `platform/linux` WebKitGTK host (`-tags vitra_native`) | Done |
| Clipboard + open-file dialog (GTK) | Done |
| Navigation allowlist (local asset server only) | Done |
| GTK menu bar + status-icon tray | Done |
| `vitra doctor/new/dev/build` | Done |
| `platform/darwin`, `platform/windows` DesktopHost stubs | Explicit unsupported |
| Eval-driven invoke E2E (`make e2e`) | Done |

## Security invariants preserved

1. Frontend never stamps caller identity — the host does.
2. Commands still require explicit grants; demo grants are narrow.
3. External navigations are denied by default and do not keep bridge authority.
4. Unsupported OS features return `platform.ErrUnsupported`, never silent success.

## Build

```bash
# Kernel + stubs (CI default)
go test ./...

# Native Linux host
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
CGO_ENABLED=1 go run -tags vitra_native ./example/competitive
# headless demo
VITRA_DEMO_SECONDS=3 xvfb-run -a make demo
# invoke round-trip through the capability gateway
make e2e
```

## Honest gap vs Wails 3

Vitra is now a **secure runtime you can run** on Linux, including menu bar and
tray. Wails still leads on cross-OS host maturity (macOS/Windows adapters) and
template ecosystem. Vitra leads on capability-oriented authority and
inspectable grants. Closing the remaining host gap is mechanical adapter work
on the same `app.DesktopHost` contract.
