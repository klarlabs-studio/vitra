# vitra

**A secure, capability-oriented desktop application runtime for Go + web frontends.**

[![CI](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml/badge.svg)](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/go.klarlabs.de/vitra.svg)](https://pkg.go.dev/go.klarlabs.de/vitra)

Secure kernel **plus** runnable DesktopHost adapters on Linux, Darwin, and
Windows. Capabilities are explicit; the frontend is untrusted; identity is
stamped by the native bridge.

---

## Why Vitra?

Web UI gives teams an exceptional rendering ecosystem. Native Go code holds
filesystem, process, network, and OS access. Frameworks become risky when that
seam is invisible.

Vitra makes the seam a first-class architectural boundary:

```text
Web Frontend (untrusted)
        │ window.vitra.invoke
        ▼
Native bridge (host-stamped identity)
        ▼
Capability Gateway  — origin · window · permission · scope
        │ authorized invocation
        ▼
Vitra Runtime (trusted) — commands · lifecycle · plugins · workers
```

A new window receives **no** privileged capability merely because it belongs
to the application. Commands are explicitly registered. Grants are narrow,
inspectable, and enforced at runtime.

See [`docs/intent.md`](docs/intent.md) for the full product charter and
[`docs/spikes/competitive.md`](docs/spikes/competitive.md) for the desktop host.

## Install

```bash
go get go.klarlabs.de/vitra
go install go.klarlabs.de/vitra/cmd/vitra@latest
```

Linux native host dependencies:

```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev pkg-config
```

## 60-Second Tour (kernel)

```go
rt, _ := vitra.New(vitra.Config{AppID: "com.example.demo"})
rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal)

grant, _ := domain.NewCapabilityGrant(
    "project-files",
    "read project files",
    []domain.WindowID{"main"},
    []domain.Origin{domain.OriginPackagedLocal},
    []domain.PermissionSpec{{
        Name: "fs.read",
        PathScope: &domain.PathScope{
            Allow: []string{"/project/**"},
            Deny:  []string{"/project/.secrets/**"},
        },
    }},
)
rt.RegisterGrant(grant)

cmd, _ := domain.NewCommandDefinition("project.open", "Open project", "fs.read")
rt.RegisterCommand(cmd, myExecutor)

caller, _ := rt.CallerFor("main")
result, err := rt.Invoke(ctx, domain.InvocationRequest{
    Caller:       caller,
    Command:      "project.open",
    Input:        "/project/app",
    ResourcePath: "/project/app",
})
```

Runnable walkthrough: `go run ./example/quickstart`

## Desktop app (competitive)

```bash
vitra new myapp
cd myapp
CGO_ENABLED=1 go run -tags vitra_native .
# or: vitra doctor && vitra dev
```

Headless demo on Linux:

```bash
VITRA_DEMO_SECONDS=3 xvfb-run -a make demo
```

## CLI

```bash
vitra version
vitra doctor
vitra new ./myapp
vitra dev
vitra build
vitra package --out dist/ [--format dir|deb|rpm-dir|rpm|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--icon path] [--maintainer name] [--description text]
vitra generate typescript --out frontend/vitra-client.ts
vitra update-apply --manifest m.json --artifact a.bin --pubkey <hex> --dest ./vitra-app
vitra inspect capabilities
```

## Architecture

```
vitra (root)     Runtime facade
app/             Desktop application runtime (assets + host + gateway)
bridge/          Injected frontend preload
platform/        OS adapters (linux WebKitGTK, darwin WKWebView, windows Win32+WebView2)
domain/          Aggregates + ports (stdlib only)
application/     Use cases
inmemory/        Default adapters
cmd/vitra/       Developer CLI
```

Dependency direction: `domain` ← `application` ← `inmemory` ← `vitra` ← `app` ← your code.

Details: [`docs/architecture-ddd.md`](docs/architecture-ddd.md).

## Status

| Track | Status |
|-------|--------|
| Secure runtime kernel (Phase 1) | Done |
| Platform spikes + IPC (Phase 0) | Contracts done; competitive host uses versioned invoke envelopes |
| Desktop completeness contracts (Phase 2) | Contracts done |
| Plugin SDK (Phase 3) | Contracts + Runtime wiring; competitive binds dialog + scoped FS |
| Distribution (Phase 4) | Specs + Linux stage/`.deb`/`.rpm`/AppDir/AppImage + Windows `win-dir`/`wix`/`nsis-dir`/`msi`/`nsis` + Darwin `app-dir`/`dmg` + signed update apply |
| Isolation / enterprise (Phase 5) | Contracts + Runtime policy/audit/workers + SIEM JSONL/CEF exporters + MDM JSON policy docs |
| **Competitive Linux WebView host** | **Done** (`-tags vitra_native`) |
| Linux menu bar + tray menus | **Done** |
| Invoke E2E (`make e2e`) | **Done** (also in CI: Native Linux E2E) |
| Host→frontend events | **Done** (`vitra.on` / `App.Emit`) |
| Multi-window App API | **Done** (`OpenWindow` / `CloseWindow`; quit on last native destroy) |
| Linux file drag-drop | **Done** (grant-gated GTK URI drops) |
| Darwin file drag-drop | **Done** (NSFilenamesPboardType drops) |
| Windows file drag-drop | **Done** (WM_DROPFILES on HWND) |
| Linux window chrome | **Done** (title, size, maximize, fullscreen, keep-above, minimize, hide, icon) |
| Linux menu accelerators | **Done** (in-window `MenuItem.Shortcut`; not global) |
| Linux OpenURL | **Done** (grant-gated `xdg-open` for http(s)/mailto) |
| Deep-link argv / secondary handoff | Linux + Darwin + Windows |
| Save dialog + single-instance lock | Linux + Darwin + Windows dialogs; single-instance Linux + Darwin + Windows |
| Linux xdg URL-scheme registration | Linux + Darwin + Windows (`vitra register-scheme`) |
| Linux xdg MIME file associations | Linux + Darwin + Windows (`vitra register-files`) |
| Darwin WKWebView | Competitive DesktopHost parity (`-tags vitra_native`) |
| Windows WebView2 | **Done** (Navigate/Eval/message via WebView2Loader; Evergreen Runtime required) |
| Windows global shortcuts | **Done** (`RegisterHotKey`) |
| Darwin global shortcuts | **Done** (`RegisterEventHotKey`; Ctrl→Command) |
| Linux global shortcuts | **Done on X11** (`XGrabKey`); unsupported on Wayland (use in-window `MenuItem.Shortcut`) |

## Development

```bash
make test
make build-native   # requires WebKitGTK
make demo           # xvfb competitive example
make e2e            # Eval→invoke→gateway round-trip
make test-native    # GTK window chrome + menu accelerators under xvfb
make check
```
