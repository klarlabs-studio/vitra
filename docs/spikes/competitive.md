# Competitive desktop runtime

This spike turns Vitra from a kernel-only stack into a **runnable** secure
desktop runtime on Linux (WebKitGTK), with API-compatible Darwin/Windows hosts
and a developer CLI that matches Wails-class DX: `new` / `dev` / `build` /
`doctor`.

## What ships

| Surface | Status |
|---------|--------|
| `app.App` — assets + WebView + invoke→gateway | Done (multi-window `OpenWindow` / `CloseWindow`; quit on last native destroy) |
| `bridge.PreloadJS` — `window.vitra.invoke` | Done |
| `platform/linux` WebKitGTK host (`-tags vitra_native`) | Done |
| Clipboard + open/save file dialogs | Clipboard Done (Linux GTK, Darwin pbcopy/pbpaste, Windows PowerShell); dialogs Linux GTK + Darwin NSOpen/SavePanel + Windows GetOpen/SaveFileName |
| Single-instance lock (flock) | Done (Linux + Darwin) |
| Deep-link argv + secondary-instance socket handoff | Done (Linux + Darwin) |
| Tray context menu (status-icon popup) | Done (Linux GTK; Darwin NSStatusItem; Windows Shell_NotifyIcon) |
| Navigation allowlist (local asset server only) | Done |
| GTK menu bar + status-icon tray | Done |
| `vitra doctor/new/dev/build` | Done |
| `platform/darwin` WKWebView host (`-tags vitra_native`) | DesktopHost complete for competitive parity (global shortcuts still TBD) |
| Linux xdg URL-scheme registration (`RegisterURLScheme`) | Done |
| Darwin URL-scheme registration (`RegisterURLScheme`) | Done (helper `.app` + lsregister) |
| Linux xdg MIME file associations (`RegisterFileAssociations`) | Done |
| Darwin MIME file associations (`RegisterFileAssociations`) | Done (helper `.app` CFBundleDocumentTypes) |
| Linux file drag-drop (`dragdrop.receive`) | Done (GTK URI drops + inject helper) |
| Darwin file drag-drop (`dragdrop.receive`) | Done (NSFilenamesPboardType + inject helper) |
| Windows file drag-drop (`dragdrop.receive`) | Done (WM_DROPFILES + inject helper) |
| In-window menu accelerators (`MenuItem.Shortcut`) | Done (Linux GTK; Darwin NSMenu Ctrl→Command; Windows HACCEL) |
| `platform/windows` DesktopHost | Win32 + WebView2 Navigate/Eval/message (WebView2Loader + Evergreen); chrome/dialogs/menu/tray/drag-drop/OpenURL/clipboard/SI/deep-link/scheme/files |
| Windows window chrome (`window.chrome`) | Done (Win32 title, size, maximize, fullscreen, topmost, minimize, hide, icon) |
| Windows open/save file dialogs | Done (GetOpenFileName / GetSaveFileName) |
| Windows menu bar + tray | Done (CreateMenu/HACCEL; Shell_NotifyIcon + TrackPopupMenu) |
| Windows URL-scheme registration (`RegisterURLScheme`) | Done (HKCU Classes `.reg`) |
| Windows MIME file associations (`RegisterFileAssociations`) | Done (HKCU ProgID + MIME `.reg`) |
| Eval-driven invoke E2E (`make e2e`) | Done (CI: Native Linux E2E) |
| Host→frontend events (`vitra.on` / `App.Emit`) | Done |
| Linux window chrome (`window.chrome`) | Done (GTK title, size, maximize, fullscreen, keep-above, minimize, hide, icon) |
| Darwin window chrome (`window.chrome`) | Done (NSWindow title, size, zoom, fullscreen, floating, miniaturize, hide, miniwindow icon) |
| OpenURL (`browser.open`) | Done (Linux `xdg-open`, Darwin `open` for http(s)/mailto) |

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

Vitra is now a **secure runtime you can run** on Linux, including menu bar,
tray with context menu, save dialogs, single-instance locking, deep-link argv handoff, and grant-gated desktop services (chrome shares the kernel gateway). Wails still leads on cross-OS host maturity (macOS/Windows adapters) and
template ecosystem. Vitra leads on capability-oriented authority and
inspectable grants. Closing the remaining host gap is mechanical adapter work
on the same `app.DesktopHost` contract.
