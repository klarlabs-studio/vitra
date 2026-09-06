# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Competitive desktop runtime: `app.App` binds the secure kernel to a native
  WebView host; `bridge` injects `window.vitra.invoke` with host-stamped identity.
- Linux WebKitGTK host (`platform/linux`, build tag `vitra_native`) with
  clipboard, open-file dialog, navigation policy, and GTK main loop.
- Darwin/Windows `DesktopHost` stubs with explicit `ErrUnsupported` feature matrices.
- CLI DX: `vitra new`, `vitra dev`, `vitra build`, upgraded `vitra doctor`.
- Competitive demo: `example/competitive` (run under xvfb with `make demo`).
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
