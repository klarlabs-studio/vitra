# Competitive desktop runtime

This spike turns Vitra from a kernel-only stack into a **runnable** secure
desktop runtime on Linux (WebKitGTK), with API-compatible Darwin/Windows hosts
and a developer CLI that matches Wails-class DX: `new` / `dev` / `build` /
`doctor`.

## What ships

| Surface | Status |
|---------|--------|
| `app.App` — assets + WebView + invoke→gateway | Done (multi-window `OpenWindow` / `CloseWindow`; quit on last native destroy); official `vitra.app` (`app.quit`) + scaffold/`generate` stubs |
| `bridge.PreloadJS` — `window.vitra.invoke` | Done |
| `platform/linux` WebKitGTK host (`-tags vitra_native`) | Done |
| Clipboard + open/save/directory/message dialogs | Clipboard Done (Linux GTK, Darwin pbcopy/pbpaste, Windows PowerShell); dialogs Linux GTK + Darwin NSOpen/SavePanel/NSAlert + Windows GetOpen/SaveFileName/IFileOpenDialog/MessageBox; open/save accept title/defaultPath/filters |
| Single-instance lock (flock) | Done (Linux + Darwin + Windows) |
| Deep-link argv + secondary-instance socket handoff | Done (Linux + Darwin + Windows) |
| Tray context menu (status-icon popup) | Done (Linux GTK; Darwin NSStatusItem; Windows Shell_NotifyIcon) |
| Navigation allowlist (local asset server only) | Done |
| GTK menu bar + status-icon tray | Done; official `vitra.menu` (`menu.set` + `menu.action`) and `vitra.tray` (`tray.set` + `tray.action`) + scaffold/`generate` stubs |
| `vitra doctor/new/dev/build` | Done |
| `platform/darwin` WKWebView host (`-tags vitra_native`) | DesktopHost complete for competitive parity |
| Darwin global shortcuts (`shortcut.global`) | Done (`RegisterEventHotKey`; Ctrl→Command); official `vitra.shortcut` plugin + scaffold/`generate` stubs |
| Linux global shortcuts (`shortcut.global`) | Done on X11 (`XGrabKey`); unsupported on Wayland; official `vitra.shortcut` plugin + scaffold/`generate` stubs |
| Linux xdg URL-scheme registration (`RegisterURLScheme`) | Done |
| Darwin URL-scheme registration (`RegisterURLScheme`) | Done (helper `.app` + lsregister) |
| Linux xdg MIME file associations (`RegisterFileAssociations`) | Done |
| Darwin MIME file associations (`RegisterFileAssociations`) | Done (helper `.app` CFBundleDocumentTypes) |
| Linux file drag-drop (`dragdrop.receive`) | Done (GTK URI drops + inject helper); official `vitra.dragdrop` plugin + scaffold/`generate` stubs |
| Darwin file drag-drop (`dragdrop.receive`) | Done (NSFilenamesPboardType + inject helper); official `vitra.dragdrop` plugin + scaffold/`generate` stubs |
| Windows file drag-drop (`dragdrop.receive`) | Done (WM_DROPFILES + inject helper); official `vitra.dragdrop` plugin + scaffold/`generate` stubs |
| In-window menu accelerators (`MenuItem.Shortcut`) | Done (Linux GTK; Darwin NSMenu Ctrl→Command; Windows HACCEL) |
| `platform/windows` DesktopHost | Win32 + WebView2 Navigate/Eval/message; chrome/dialogs/menu/tray/drag-drop/global shortcuts/OpenURL/clipboard/SI/deep-link/scheme/files |
| Windows global shortcuts (`shortcut.global`) | Done (`RegisterHotKey` → action handler); official `vitra.shortcut` plugin + scaffold/`generate` stubs |
| Windows window chrome (`window.chrome`) | Done (Win32 title, size, maximize, fullscreen, topmost, minimize, hide, icon); official `vitra.window` plugin + scaffold/`generate` stubs |
| Windows open/save/directory file dialogs | Done (GetOpenFileName / GetSaveFileName / IFileOpenDialog FOS_PICKFOLDERS) |
| Windows menu bar + tray | Done (CreateMenu/HACCEL; Shell_NotifyIcon + TrackPopupMenu); official `vitra.menu` / `vitra.tray` plugins + scaffold/`generate` stubs |
| Windows URL-scheme registration (`RegisterURLScheme`) | Done (HKCU Classes `.reg`) |
| Windows MIME file associations (`RegisterFileAssociations`) | Done (HKCU ProgID + MIME `.reg`) |
| Eval-driven invoke E2E (`make e2e`) | Done (CI: Native Linux E2E) |
| Host→frontend events (`vitra.on` / `App.Emit`) | Done |
| Linux window chrome (`window.chrome`) | Done (GTK title, size, maximize, fullscreen, keep-above, minimize, hide, icon); official `vitra.window` plugin + scaffold/`generate` stubs |
| Darwin window chrome (`window.chrome`) | Done (NSWindow title, size, zoom, fullscreen, floating, miniaturize, hide, miniwindow icon); official `vitra.window` plugin + scaffold/`generate` stubs |
| OpenURL (`browser.open`) | Done (Linux `xdg-open`, Darwin `open`, Windows `cmd start` for http(s)/mailto); official `vitra.browser` plugin + scaffold/`generate` stubs |
| Host info (`os.info`) | Done (stdlib GOOS/GOARCH/family + locale env); official `vitra.os` plugin + scaffold/`generate` stubs |
| Notifications (`notifications.show`) | Done (title+body; Linux D-Bus Notifications, Darwin NSUserNotification, Windows tray balloon); official `vitra.notification` plugin + scaffold/`generate` stubs |
| Open path (`path.open`) | Done (absolute local paths via `xdg-open` / `open` / `explorer`; PathScope-gated); official `vitra.path` plugin + scaffold/`generate` stubs |

## Security invariants preserved

1. Frontend never stamps caller identity — the host does.
2. Commands still require explicit grants; demo grants are narrow.
3. External navigations are denied by default and do not keep bridge authority.
4. Unsupported OS features return `platform.ErrUnsupported`, never silent success.

## Build

```bash
# Kernel + stubs (CI default)
go test ./...

# Native DesktopHost (Linux CI; also Darwin WKWebView / Windows WebView2)
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev   # Linux
CGO_ENABLED=1 go run -tags vitra_native ./example/competitive
# headless demo (Linux/xvfb)
VITRA_DEMO_SECONDS=3 xvfb-run -a make demo
# invoke round-trip through the capability gateway
make e2e
```

The competitive example selects `platform/linux`, `platform/darwin`, or
`platform/windows` from `GOOS` so the same demo binary path exercises each
DesktopHost adapter.

## Honest gap vs Wails 3

Vitra is a **secure runtime you can run** on Linux, Darwin, and Windows with
API-compatible `app.DesktopHost` adapters (WebKitGTK / WKWebView / Win32+WebView2),
including menus, tray, dialogs, chrome, drag-drop, deep links, and grant-gated
desktop services. Global OS hotkeys ship on Windows, Darwin, and Linux/X11;
Wayland keeps in-window accelerators only (no portable global hotkey API).

Wails still leads on **template ecosystem** and **release polish** (richer
presets beyond Vitra’s `vanilla`/`vite`/`react`/`svelte`/`vue`/`solid`/`preact`/`lit`
starters; interactive Flathub/Snap Store login automation; turnkey notarize
credential bootstrap). Vitra leads on capability-oriented authority and
inspectable grants. Core DesktopHost, installer stage+fold (`deb` / `.rpm` /
`.snap` / `.flatpak` / AppImage / WiX·MSI / NSIS / `.app`·DMG), SIEM/MDM ports,
and X11 global-shortcut parity are on main — Wayland global hotkeys remain
intentionally unsupported. `vitra new --template vite|react|svelte|vue|solid|preact|lit|alpine|htmx|angular|qwik`
scaffolds Vite frontends; `vitra
dev` / `vitra build` enable `-tags vitra_native` on Linux, Darwin, and Windows.
Fold still needs host tools (`appimagetool`, `rpmbuild`, `snapcraft`,
`flatpak-builder`, candle/light, makensis, `hdiutil`) or env overrides;
`Spec.Sign` / `SigningIdentityRef` validate refs and `vitra package --sign`
prints a plan (`PlanSign`); `--sign-execute` runs host tools via
`ExecuteSign` (Darwin codesign + optional notarytool/stapler follow-ups,
Windows signtool, Linux dpkg-sig/rpmsign/snapcraft/gpg; `secret:` refs expand
from `VITRA_SECRET_*` without leaking into plans). `vitra notary-setup` /
Darwin `PlanSign` prep print `notarytool store-credentials` guidance (not
executed). `--publish` prints a Snap Store / Flathub `PlanPublish` (plan only);
`--publish-execute` runs Executable steps (`snapcraft upload` /
`flatpak-builder`) and never interactive login or Flathub `gh pr create`.
Flatpak stage prefers xdg-desktop-portal talk-names over `--filesystem=home`.
Update channel *client* fetch
(`updater.Fetcher` / `vitra update-check` / `update-apply --base-url`) verifies
signed HTTP(S) manifests and can fetch + install artifacts; publish-side
`vitra update-keygen` / `update-sign` / `update-stage` complete the operator
loop without hosting a CDN. Audit SIEM exporters (`JSONLSink` / `CEFSink`) and MDM JSON
policy documents (`policy.LoadDocument`) plug into Phase 5 ports.
