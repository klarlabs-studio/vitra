# vitra

**A secure, capability-oriented desktop application runtime for Go + web frontends.**

[![CI](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml/badge.svg)](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/go.klarlabs.de/vitra.svg)](https://pkg.go.dev/go.klarlabs.de/vitra)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/klarlabs-studio/vitra/badge)](https://scorecard.dev/viewer/?uri=github.com/klarlabs-studio/vitra)

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

**Documentation: [klarlabs-studio.github.io/vitra](https://klarlabs-studio.github.io/vitra/)** (sources in [`docs/`](docs)).

See [`docs/security.md`](docs/security.md) for the security model and its known
limits, [`docs/intent.md`](docs/intent.md) for the full product charter, and
[`docs/spikes/competitive.md`](docs/spikes/competitive.md) for the desktop host.

## Install

```bash
go get go.klarlabs.de/vitra
go install go.klarlabs.de/vitra/cmd/vitra@latest
```

Native hosts: macOS 11 or later (WKWebView), Windows 10/11 with the
WebView2 Evergreen Runtime, and Linux with WebKitGTK 4.1.

Linux native host dependencies:

```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev pkg-config
```

## 60-Second Tour (kernel)

```go
rt, _ := vitra.New(vitra.Config{AppID: "com.example.demo"})
rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal) // no authority yet

// An explicit, typed command. Input is decoded strictly into OpenReq; the
// handler learns which window called it and which path was authorized.
type OpenReq struct {
    Path string `json:"path"`
}
func (r OpenReq) ResourcePath() string { return r.Path } // must match the checked path

vitra.Register(rt, vitra.Command[OpenReq, string]{
    Name:       "project.open",
    Permission: "fs.read",
    Handler: func(ctx context.Context, inv domain.Invocation, req OpenReq) (string, error) {
        return "opened " + req.Path + " for " + string(inv.Caller.Window), nil
    },
})

// Grant the main window fs.read under /project, except its secrets.
grant, _ := domain.NewCapabilityGrant(
    "project-files", "read project files",
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
```

The frontend calls it through a generated, typed client:

```bash
vitra generate typescript --app . --out frontend/vitra-client.ts
```

```ts
// projectOpen(input: OpenReq, resourcePath?: string): Promise<string>
await client.projectOpen({ path: "/project/app" }, "/project/app"); // "opened /project/app for main"
await client.projectOpen({ path: "/project/.secrets/key" }, "/project/.secrets/key"); // denied: path_denied
```

Runnable walkthrough: `go run ./example/quickstart`

## See it work

![The notes demo: a note is edited and saved, then nine attacks from the page are each refused and shown in the live audit log](docs/assets/notes-demo.gif)

```bash
make notes   # or: CGO_ENABLED=1 go run -tags vitra_native ./example/notes
```

[`example/notes`](example/notes) is a Markdown notes app. Its window may read
one vault folder (except `.private/`) and write `*.md` files in it, nothing
else. The **Try to break it** panel attacks the app for real: reading
`/etc/hosts`, climbing out with `../`, saving a shell script, embedding a
remote page, forging a bridge call from a sandboxed frame. The live audit
log shows each one refused, with the rule that refused it.

## Desktop app

```bash
vitra new myapp
# or: vitra new myapp --template vite|react|svelte|vue
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
# vitra new ./myapp --template vite|react|svelte|vue
vitra dev
vitra build
vitra package --out dist/ [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--icon path] [--maintainer name] [--description text] [--homepage url] [--categories list] [--license spdx] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref] [--publish [--publish-execute]]
vitra generate typescript --out frontend/vitra-client.ts
vitra notary-setup [--profile name]
vitra update-keygen [--out keys/]
vitra update-sign --artifact a.bin --app-id com.example.app --version 1.0.0 --privkey file:keys/priv.key --out m.json
vitra update-stage --out dist/updates --manifest m.json --artifact a.bin
vitra update-check --base-url https://updates.example/ --app-id com.example.app --channel stable --pubkey <hex>
vitra update-apply --base-url https://updates.example/ --app-id com.example.app --channel stable --current-version 1.0.0 --pubkey <hex> --dest ./vitra-app
vitra update-apply --manifest m.json --artifact a.bin --app-id com.example.app --current-version 1.0.0 --pubkey <hex> --dest ./vitra-app
vitra inspect capabilities
```

## Architecture

```
vitra (root)     Runtime facade: grants, typed commands, windows, events
app/             Desktop app runner (assets + native host + gateway)
domain/          Grants, commands, path scopes, denials (stdlib only)
desktop/         Capability-gated desktop services (fs, dialogs, clipboard, …)
platform/        OS hosts (linux WebKitGTK, darwin WKWebView, windows WebView2)
plugin/          Plugin SDK + official plugins
policy/ audit/ updater/ worker/   Enterprise policy, audit sinks, signed updates, workers
cmd/vitra/       Developer CLI

internal/        Not importable by apps:
  application/   Use cases
  inmemory/      Default repository adapters
  ipc/ bridge/   Wire envelope + injected frontend preload
  bindings/      TypeScript client generator
  packaging/ provenance/   Installer staging, SBOM metadata (CLI only)
  platform/null/ Headless host for tests
```

Dependency direction: `domain` ← `internal/application` ← `internal/inmemory` ← `vitra` ← `app` ← your code.

Details: [`docs/architecture-ddd.md`](docs/architecture-ddd.md).

## Status

| Track | Status |
|-------|--------|
| Secure runtime kernel (Phase 1) | Done |
| Platform spikes + IPC (Phase 0) | Contracts done; competitive host uses versioned invoke envelopes |
| Desktop completeness contracts (Phase 2) | Contracts done |
| Plugin SDK (Phase 3) | Contracts + Runtime wiring; competitive binds dialog + scoped FS |
| Distribution (Phase 4) | Specs + Linux stage/`.deb`/`.rpm`/`.snap`/`.flatpak`/AppDir/AppImage + Windows `win-dir`/`wix`/`nsis-dir`/`msi`/`nsis` + Darwin `app-dir`/`dmg` + signed update apply |
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

## Stability

Vitra is pre-1.0: minor releases may break things, and each one says how to upgrade. See [compatibility and the road to 1.0](https://klarlabs-studio.github.io/vitra/compatibility) for what is covered and what is still open.

## Security

The frontend is untrusted by design, so a gateway bypass, path scope escape,
spoofed caller identity, or update verification failure is a vulnerability.
Report it privately: see [SECURITY.md](SECURITY.md). Contributions follow the
[Code of Conduct](CODE_OF_CONDUCT.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Development

```bash
make test
make build-native   # requires WebKitGTK
make demo           # xvfb competitive example
make e2e            # Eval→invoke→gateway round-trip
make test-native    # GTK window chrome + menu accelerators under xvfb
make check
```
