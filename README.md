# vitra

**A secure, capability-oriented desktop application runtime for Go + web frontends.**

[![CI](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml/badge.svg)](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/go.klarlabs.de/vitra.svg)](https://pkg.go.dev/go.klarlabs.de/vitra)

Secure kernel **plus** a runnable Linux WebView host. Capabilities are explicit;
the frontend is untrusted; identity is stamped by the native bridge.

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
vitra inspect capabilities
```

## Architecture

```
vitra (root)     Runtime facade
app/             Desktop application runtime (assets + host + gateway)
bridge/          Injected frontend preload
platform/        OS adapters (linux WebKitGTK, darwin/windows stubs)
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
| Platform spikes + IPC (Phase 0) | Done |
| Desktop completeness contracts (Phase 2) | Done |
| Plugin SDK (Phase 3) | Done |
| Distribution (Phase 4) | Done |
| Isolation / enterprise (Phase 5) | Done |
| **Competitive Linux WebView host** | **Done** (`-tags vitra_native`) |
| Linux menu bar + tray | **Done** |
| Invoke E2E (`make e2e`) | **Done** |
| Darwin WKWebView / Windows WebView2 | Stubs (explicit unsupported) |

## Development

```bash
make test
make build-native   # requires WebKitGTK
make demo           # xvfb competitive example
make e2e            # Eval→invoke→gateway round-trip
make check
```
