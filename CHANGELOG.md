# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
