# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `Artifact.Signed` is set after successful `ExecuteSign`; `RefreshArtifactDigest` recomputes SHA-256 for file artifacts (directory bundles keep prior digest).
- `vitra new --template preact`: Vite + Preact + TypeScript starter (`App.tsx`, `@preact/preset-vite`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `secret:` signing refs resolve at `ExecuteSign` via `VITRA_SECRET_<NAME>` (path separators → `_`; plan argv stays opaque `<secret:…>`).
- Store publish dry-run: `packaging.PlanPublish` + `vitra package --publish` prints Snap Store (`snapcraft login|register|upload`) and Flathub (`flatpak-builder` / PR) step plans without invoking store tools.
- `vitra update-apply --base-url`: fetch signed channel manifest + artifact via `updater.Fetcher`, then verify and atomically install (same invariant 9 path as local `--manifest`/`--artifact`).
- `vitra new --template solid`: Vite + SolidJS + TypeScript starter (`App.tsx`, `vite-plugin-solid`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Packaging sign execution: `packaging.ExecuteSign` runs PlanSign argv (expands `${ENV}`, rejects `secret:` refs); `vitra package --sign-execute` [--sign-follow-ups] invokes host tools; `vitra doctor` reports `stapler`.
- Update channel HTTP client: `updater.ChannelSource` / `Fetcher` fetch `{base}/{app}/{channel}/manifest.json` (+ artifact); `vitra update-check` verifies the signed manifest without installing. Hosted CDN remains out of scope.
- `vitra new --template vue`: Vite + Vue 3 + TypeScript starter (`App.vue` script setup, `@vitejs/plugin-vue`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Linux package sign dry-run: `PlanSign` supports `.deb` (`dpkg-sig`), `.rpm` (`rpmsign`), `.snap` (`snapcraft upload`), and AppImage/Flatpak (`gpg --detach-sign`); `vitra doctor` reports `gpg` / `dpkg-sig` / `rpmsign`.
- Darwin notarize dry-run: `PlanSign` for `.app`/`.dmg` prints `notarytool submit` + `stapler staple` follow-up argv (still never executed); profile from `NOTARYTOOL_PROFILE` or `keychain:` signing ref.
- `vitra new --template svelte`: Vite + Svelte 5 + TypeScript starter (`App.svelte`, `@sveltejs/vite-plugin-svelte`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template react`: Vite + React + TypeScript starter (`App.tsx`, `@vitejs/plugin-react`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Linux Flatpak packaging: `vitra package --format flatpak-dir|flatpak` stages `files/` + `metadata` + `manifest.yml` (`BuildFlatpakDir`) and folds via `VITRA_FLATPAK_BUILDER` / `flatpak-builder` (`<stage> <out.flatpak>`); `vitra doctor` reports the tool.
- Linux Snap packaging: `vitra package --format snap-dir|snap` stages a snap prime tree (`BuildSnapDir`: FreeDesktop payload + `meta/snap.yaml`) and folds via `snapcraft pack` (`VITRA_SNAPCRAFT`); `vitra doctor` reports `snapcraft`.
- `vitra new --template vanilla|vite`: default remains vanilla HTML; `vite` scaffolds a Vite+TypeScript frontend (`package.json`, `src/main.ts`) embedding `frontend/dist` with a starter dist so `vitra dev` works before the first `npm run build`.
- Packaging sign dry-run: `vitra package --sign --signing-identity <ref>` prints a `PlanSign` argv plan (codesign / signtool) without executing tools or leaking secret values; `vitra doctor` reports `codesign` / `signtool` / `notarytool`.
- Linux RPM packaging: `vitra package --format rpm-dir|rpm` stages an rpmbuild `_topdir` (`BuildRPMDir`: FreeDesktop payload + `SPECS/*.spec`) and folds via `rpmbuild` (`VITRA_RPMBUILD`); `vitra doctor` reports `rpmbuild`.
- Official `vitra.clipboard` plugin: declares `clipboard.read`/`clipboard.write`; scaffold, `vitra generate typescript`, packaging provenance inventory, and competitive demo register + bind clipboard executors alongside `fs`/`dialog`.
- Cross-host process identity: Darwin `SetProgramName` → `NSProcessInfo.setProcessName`; Windows → `SetCurrentProcessExplicitAppUserModelID`; WiX shortcuts emit `System.AppUserModel.ID` from `AppID`; competitive demo sets `vitra-competitive` on all hosts.
- Packaging provenance: `WithModulesFromBuildInfo` fills Go module inventory from the packaged binary (or CLI build info fallback); plugin/capability rows are derived from official plugin Manifests via `WithPluginInventory`.
- `vitra new` scaffold registers official `fs` + `dialog` + `clipboard` plugins (bound dialog/fs/clipboard executors + scoped grant), emits `frontend/vitra-client.ts`, and documents `vitra generate typescript` / `vitra package`.
- Linux host WM_CLASS: `vitra_gtk_init` calls `g_set_prgname` / `gdk_set_program_class` (default `filepath.Base(os.Args[0])`, overridable via `Host.SetProgramName`) so docks match FreeDesktop `StartupWMClass`.
- Packaging `Spec.Description` / `vitra package --description`: sets Debian control extended Description and FreeDesktop `Comment=` on dir/AppDir/deb `.desktop` files (default short Vitra blurb).
- Linux `.desktop` files emit `StartupWMClass=<binary basename>` (dir / AppDir / deb) so docks can bind the running window to the launcher.
- Linux package icons: AppDir stages `.DirIcon` + `usr/share/icons/hicolor/{256x256|scalable}/apps/`; dir stage and `.deb` also emit the hicolor theme path (deb keeps `usr/share/pixmaps/` too).
- Packaging `Spec.Maintainer` / `vitra package --maintainer`: sets Debian control Maintainer (default `Vitra Packaging <vitra@klarlabs.de>`) and WiX Manufacturer / NSIS Publisher when provided.
- Packaging `SigningIdentityRef` validation: refs must use `env:` / `keychain:` / `file:` / `secret:` prefixes; PEM / `PRIVATE KEY` material is rejected (invariant 10).
- WiX Desktop shortcut: `BuildWiXDir` emits a `DesktopFolder` shortcut (with optional `Icon` when `--icon` is set), matching NSIS Desktop UX.
- NSIS desktop shortcut: `BuildNSISDir` creates `$DESKTOP\<Name>.lnk` (with icon when `--icon` is set) and removes it on uninstall.
- `vitra doctor` reports packaging fold-tool availability (`appimagetool`, `candle`/`light`, `makensis`, `hdiutil`) with `VITRA_*` override hints.
- WiX Start Menu shortcut: `BuildWiXDir` emits `ProgramMenuFolder` shortcut + uninstall `RemoveFolder`, and a deterministic `UpgradeCode` GUID from `AppID` (optional shortcut `Icon` when `--icon` is set).
- Docs: competitive/phase2/phase4/README status aligned with shipped DesktopHost + installer folds; honest gap reframed to templates/release polish (Wayland global hotkeys still unsupported).
- NSIS installer UX: `BuildNSISDir` emits `WriteUninstaller`, HKCU Add/Remove Programs registry keys (`DisplayName`/`UninstallString`/…), and cleans them on uninstall; optional `DisplayIcon` when `--icon` is set.
- Linux `.deb` package icons: `Spec.IconPath` / `--icon` stages `usr/share/pixmaps/<name>.<ext>` and sets `Icon=` on the FreeDesktop `.desktop` entry.
- Darwin DMG drag-install layout: FoldDMG stages `.app` + `/Applications` symlink in the image root before `hdiutil create`.
- `app.DesktopHost` includes `RegisterFileAssociations` and `InjectFileDrop` (all OS adapters already implemented); competitive demo uses the shared interface.
- Windows package icons: `Spec.IconPath` / `--icon` stages into `bin/` for `win-dir`; WiX emits `Icon`/`ARPPRODUCTICON`, NSIS shortcuts use the icon file.
- TypeScript bindings: `vitra generate typescript` emits `createEvents` / `on*` helpers from plugin `Contribution.Events` (e.g. `onFsChanged`); Darwin/Windows stub `ErrUnsupported` details no longer point at Linux-only hosts.
- Package icons: optional `Spec.IconPath` / `vitra package --icon` stages `.png`/`.svg`/`.icns` into Linux dir/AppDir and Darwin `.app` Resources (`CFBundleIconFile`).
- Competitive demo selects Linux / Darwin / Windows DesktopHost by `GOOS` (deep-link argv helpers included); Linux CI e2e unchanged.
- Darwin DMG fold: `vitra package --format dmg` stages a `.app` then invokes `hdiutil` (`VITRA_HDIUTIL` override); CI covers fold with a fake tool.
- Darwin `.app` stage: `vitra package --format app-dir` builds `<Name>.app/Contents/{Info.plist,MacOS/<exec>}` via `StageDarwinApp`.
- CLI `vitra dev` / `vitra build` pass `-tags vitra_native` on Darwin and Windows (not only Linux), matching DesktopHost adapters; scaffold README points at `vitra dev`.
- Linux X11 global shortcuts: `RegisterGlobalShortcut` / `UnregisterGlobalShortcut` via `XGrabKey` when not on Wayland; feature matrix stays false on Wayland / missing DISPLAY.
- SIEM/MDM hooks: `audit.JSONLSink` (NDJSON) + `audit.CEFSink` (Common Event Format) + `MultiSink`; `policy.LoadDocument` / `Document.Save` for fleet JSON; competitive demo honors `VITRA_AUDIT=jsonl|cef`, `VITRA_AUDIT_PATH`, and `VITRA_POLICY_FILE`.
- Windows MSI/NSIS fold: `vitra package --format msi|nsis` stages then invokes candle/light or makensis (`VITRA_CANDLE`/`VITRA_LIGHT`/`VITRA_MAKENSIS` overrides); CI covers fold with fake tools.
- Windows packaging stage: `vitra package --format win-dir|wix|nsis-dir` stages `bin/<Name>.exe` and emits WiX `product.wxs` / NSIS `installer.nsi` intermediates (fold with candle/light/makensis externally).
- Competitive docs: DesktopHost parity called out for Linux/Darwin/Windows; honest gap vs Wails reframed to templates/packaging and Wayland global hotkeys (not missing OS adapters).
- Darwin global shortcuts: `RegisterGlobalShortcut` / `UnregisterGlobalShortcut` via Carbon `RegisterEventHotKey` (Ctrl→Command, matching menu accelerators); stub remains explicit unsupported.
- Windows global shortcuts: `RegisterGlobalShortcut` / `UnregisterGlobalShortcut` via `RegisterHotKey` (OS-wide; requires a modifier); `ShortcutService.Register` now takes `actionID`; competitive demo binds `Ctrl+Shift+Q` → `app.quit` when supported.
- Windows WebView2 Navigate/Eval/message: `LoadLibrary(WebView2Loader.dll)` + hand-rolled COM for Navigate, ExecuteScript, chrome.webview messages, and document-start preload; bridge prefers `chrome.webview.postMessage` then webkit. HWND shell remains when the loader/runtime is absent.
- Windows file drag-drop: `EnableDragDrop` accepts `WM_DROPFILES` on the HWND; `InjectFileDrop` for demos/tests under `-tags vitra_native`.
- Windows menu bar + tray: `SetMenuBar` / `ActivateMenuAccel` via Win32 menus + HACCEL; `SetTray` / `ClearTray` via Shell_NotifyIcon + TrackPopupMenu under `-tags vitra_native`; stub remains explicit unsupported.
- Windows file dialogs: native `OpenFileDialog` / `SaveFileDialog` via GetOpenFileName / GetSaveFileName under `-tags vitra_native`; stub remains explicit unsupported.
- Windows window chrome: `ApplyWindowChrome` / `ReadWindowChrome` on the Win32 shell (title, size, maximize, fullscreen, topmost, minimize, hide, icon).
- Windows URL-scheme and file-association registration: HKCU Classes `.reg` helpers under LOCALAPPDATA with best-effort `reg import`; `vitra register-scheme` / `register-files` work on Windows.
- Windows clipboard + single-instance + deep-link: PowerShell Get/Set-Clipboard; exclusive lock file; argv/socket handoff (available without WebView2).
- Windows WebView2 host scaffold (`platform/windows`, `-tags vitra_native`): Win32 HWND shell for Open/Close/Run/Quit; Navigate/Eval stay explicit unsupported until WebView2 SDK wiring. OpenURL via `cmd /c start` works without the native host.
- Darwin URL-scheme and file-association registration: helper `.app` bundles under Application Support (`CFBundleURLTypes` / `CFBundleDocumentTypes`) with best-effort `lsregister`; `vitra register-scheme` / `register-files` work on Darwin.
- Darwin file drag-drop: `EnableDragDrop` accepts `NSFilenamesPboardType` drops on the window content view; `InjectFileDrop` for demos/tests.
- Darwin status-item tray: `SetTray` / `ClearTray` via `NSStatusItem` with context menu actions through `SetActionHandler`.
- Darwin menu bar: `SetMenuBar` builds an `NSApp` main menu with `MenuItem.Shortcut` accelerators (Ctrl maps to Command); `ActivateMenuAccel` for demos/tests.
- Darwin window chrome: `ApplyWindowChrome` / `ReadWindowChrome` on the WKWebView host (title, size, zoom, fullscreen, floating, miniaturize, hide, miniwindow icon).
- Darwin file dialogs: native `OpenFileDialog` / `SaveFileDialog` via NSOpenPanel / NSSavePanel under `-tags vitra_native`; stub remains explicit unsupported.
- Darwin single-instance + deep-link handoff: flock lock and unix-socket secondary→primary URL forwarding (mirrors Linux; available without WKWebView).
- Darwin clipboard: `ClipboardGet`/`ClipboardSet` via `pbpaste`/`pbcopy` (available without WKWebView); grant path unchanged (`clipboard.read`/`clipboard.write`).
- Darwin OpenURL: grant-gated `browser.open` launches http(s)/mailto via macOS `open` (available without WKWebView); scheme validation mirrors Linux.
- Darwin WKWebView host scaffold (`platform/darwin`, `-tags vitra_native`): NSWindow + WKWebView Open/Navigate/Eval/PostMessage/Run/Quit with script-message bridge and nav policy; non-core DesktopHost surfaces stay explicit `ErrUnsupported`. Without the tag, the stub adapter remains.
- Linux window icon: `WindowChrome.IconPath` sets the GTK window icon from a filesystem image; empty path leaves the icon unchanged.
- Quit on last native window destroy: `SetDestroyHandler` syncs `App`/`Runtime` when GTK closes a window (titlebar); last window quits without double-Quit on API `CloseWindow`.
- Multi-window `app.App`: `OpenWindow` / `CloseWindow` / `Windows`; closing the last window quits. Competitive demo honors `VITRA_SECOND_WINDOW=1`.
- Linux window chrome minimize/hide: `WindowChrome.Minimized` / `Hidden` map to GTK iconify and hide; zero-value `Hidden` stays visible (compatible with existing apply calls).
- Worker IPC transport: newline-delimited JSON `Session` (`Send`/`Recv`/`Request`) over in-memory pipes or OS-process stdio (`StdioIPC`); oversized/malformed frames fail closed.
- Linux OpenURL: grant-gated `desktop.BrowserService` (`browser.open`) launches http(s)/mailto via `xdg-open`; competitive demo honors `VITRA_OPEN_URL`.
- Linux in-window menu accelerators: `platform.MenuItem.Shortcut` (e.g. `Ctrl+Q`) binds GTK accel groups; competitive Quit uses `Ctrl+Q`. Global shortcuts remain unsupported on Wayland.
- Linux window chrome: grant-gated `desktop.WindowService` (`window.chrome`) sets GTK title, size, maximize, fullscreen, and keep-above; competitive demo honors `VITRA_WINDOW_TITLE`.
- Linux xdg MIME file associations (`Host.RegisterFileAssociations`, `vitra register-files`); competitive demo honors `VITRA_REGISTER_FILES`.
- OS worker processes: `worker.CommandRunner` execs a binary under `Supervisor` (cancel stops the process; non-zero exit is a crash).
- Linux file drag-drop: grant-gated `desktop.DragDropService`, GTK URI drops on the WebView, `InjectFileDrop` for headless demos; competitive emits `dragdrop.drop`.
- CLI `vitra update-apply` verifies a signed manifest and atomically installs the artifact (`Runtime.ApplyUpdate`, optional `--policy`).
- Competitive invoke path unified on versioned `ipc` envelopes: preload posts `protocol/kind/payload`; `app.App` uses `ipc.Bridge.DecodeInvoke` (host identity wins).
- Final `.AppImage` fold: `packaging.BuildAppImage` / `FoldAppDir` via `appimagetool` (`VITRA_APPIMAGETOOL` override); `vitra package --format appimage`.
- CLI `vitra generate typescript` emits official-plugin TypeScript client stubs (`bindings.GenerateTypeScript`).
- Host→frontend events: `Runtime.EmitEvent`, `app.App.Emit`, `vitra.on` in preload, `ipc.EncodeEvent`; competitive demo emits `demo.tick`.
- Runtime worker facade: `StartWorker` / `StopWorker` / `Workers` over in-process `worker.Supervisor` with `worker.lifecycle` audit transitions.
- Runtime audit wiring: `SetAudit` / `Audit()` emits capability, policy override, invoke, plugin, and update events; competitive demo honors `VITRA_AUDIT=1`.
- Signed update apply: `updater.ApplyInstall` (atomic replace) and `Runtime.ApplyUpdate` (policy channel gate → verify → install).
- Runtime enterprise policy wiring: `SetPolicy` / `Policy()` overlay on `Authorize` and `Invoke`; competitive demo honors `VITRA_POLICY` (+ optional `VITRA_POLICY_DENY`).
- Linux `.deb` builder (`packaging.BuildDeb`) and AppImage AppDir (`BuildAppDir`); `vitra package --format deb|appdir|dir`.
- Linux package staging: `packaging.StageLinux` + `vitra package` (app dir, `.desktop`, `provenance.json` with artifact digest).
- Grant-scoped `desktop.FileService` bound to official `fs.read`/`fs.write` in the competitive demo (PathScope enforced).
- Linux xdg URL-scheme registration (`Host.RegisterURLScheme`, `vitra register-scheme`); competitive demo honors `VITRA_REGISTER_SCHEME=1`.
- Runtime plugin wiring: `RegisterPlugin` / `BindExecutor` / `Plugins()`; competitive demo and `vitra inspect` load official `fs` + `dialog` plugins (dialog executors bound; fs unbound until host provides handlers).
- CI job **Native Linux E2E**: install WebKitGTK/GTK/xvfb, `make build-native`, and `make e2e` on `ubuntu-latest`.
- E2E retries Eval until `window.vitra` is ready and allows 20s for cold WebKit on CI.
- Linux tray context menus on the status-icon; `DesktopHost.SetTray` accepts menu items.
- Competitive demo chrome (menu/tray/dialogs/clipboard/deeplink) authorizes through the kernel gateway (`Runtime.Authorize`).
- `vitra new` scaffold no longer emits a duplicate `io/fs` import.
- Kernel version bumped to `0.3.0` (matches official plugin `MinKernel`).
- Linux deep-link argv parsing and unix-socket secondary-instance handoff.
- Linux save-file dialog and flock-based single-instance lock on `platform/linux`.
- `app.DesktopHost` now includes menu/tray/action/save/single-instance/deep-link surface.
- Competitive demo routes clipboard/dialogs/menu/tray through `desktop` permission services.
- `vitra new` emits `go.mod`; `vitra dev` watches source and restarts the app.
- Competitive desktop runtime: `app.App` binds the secure kernel to a native
  WebView host; `bridge` injects `window.vitra.invoke` with host-stamped identity.
- Linux WebKitGTK host (`platform/linux`, build tag `vitra_native`) with
  clipboard, open/save dialogs, GTK menu bar, status-icon tray + context menu,
  navigation policy, and GTK main loop.
- Eval-driven E2E (`make e2e`) proving frontend invoke → capability gateway.
- Darwin/Windows `DesktopHost` stubs with explicit `ErrUnsupported` feature matrices.
- CLI DX: `vitra new`, `vitra dev`, `vitra build`, upgraded `vitra doctor`.
- Competitive demo: `example/competitive` (run under xvfb with `make demo`).
- CI runs on all pull requests (including stacked non-`main` bases).
- Phase 5 isolation/enterprise: supervised workers with crash-safe bookkeeping,
  audit sink, enterprise policy overlay (signed updates + no dev-priv leak in production).
- Phase 4 distribution: packaging specs with signing identity refs (invariant 10),
  ed25519-signed update manifests + install plans (invariant 9), provenance/SBOM documents.
- Phase 3 plugin SDK: manifests, SemVer compatibility, permission-ownership registry (invariant 6),
  official `fs`/`dialog` plugin contracts, TypeScript binding stub generator.
- Phase 2 desktop completeness: navigation policy, window-owned event
  subscriptions (cleaned up on window close), capability-gated `desktop`
  services (menu/tray/dialog/clipboard/shortcuts/deeplinks/single-instance).
- Phase 0 architecture spikes: versioned `ipc` bridge (host identity wins over
  payload claims), `platform` feature matrix + `platform/null` stub host with
  explicit unsupported errors, spike notes in `docs/spikes/phase0.md`.
- Phase 1 secure runtime kernel: capability grants, gateway, explicit commands,
  invocation pipeline, window lifecycle, resource handles.
- In-memory port adapters and `vitra.Runtime` facade.
- CLI skeleton: `vitra version`, `vitra doctor`, `vitra inspect capabilities`.
- Product intent charter and DDD architecture docs.
- Quickstart example demonstrating grant → invoke → navigate denial.
- Klarlabs tooling: Makefile, golangci-lint, coverctl, nox, warden, shared go-ci.
