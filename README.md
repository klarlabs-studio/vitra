# vitra

**A secure, capability-oriented desktop application runtime for Go + web frontends.**

[![CI](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml/badge.svg)](https://github.com/klarlabs-studio/vitra/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/go.klarlabs.de/vitra.svg)](https://pkg.go.dev/go.klarlabs.de/vitra)

**Phase 1: secure runtime kernel.** Zero external dependencies. Standard library only.

---

## Why Vitra?

Web UI gives teams an exceptional rendering ecosystem. Native Go code holds
filesystem, process, network, and OS access. Frameworks become risky when that
seam is invisible.

Vitra makes the seam a first-class architectural boundary:

```text
Web Frontend (untrusted)
        │ typed messages
        ▼
Capability Gateway  — origin · window · permission · scope
        │ authorized invocation
        ▼
Vitra Runtime (trusted) — commands · lifecycle · diagnostics
```

A new window receives **no** privileged capability merely because it belongs
to the application. Commands are explicitly registered. Grants are narrow,
inspectable, and enforced at runtime.

See [`docs/intent.md`](docs/intent.md) for the full product charter.

## Install

```bash
go get go.klarlabs.de/vitra
go install go.klarlabs.de/vitra/cmd/vitra@latest
```

## 60-Second Tour

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

## CLI

```bash
vitra version
vitra doctor
vitra inspect capabilities
```

Exact commands are not yet contractual; the skeleton matches the DX shape in
the charter.

## Architecture

Strict DDD / hexagonal (Klarlabs house style, same spirit as [axi-go](https://github.com/klarlabs-studio/axi-go)):

```
vitra (root)     Runtime facade
domain/          Aggregates + ports (stdlib only)
application/     Use cases
inmemory/        Default adapters
cmd/vitra/       Developer CLI
```

Dependency direction: `domain` ← `application` ← `inmemory` ← `vitra` ← your code.

Details: [`docs/architecture-ddd.md`](docs/architecture-ddd.md).

## Status

| Phase | Status |
|-------|--------|
| 0 — Platform spikes (WebView + IPC) | Planned |
| 1 — Secure runtime kernel | **In progress** |
| 2 — Desktop completeness | Planned |
| 3 — Plugin SDK | Planned |
| 4 — Distribution | Planned |
| 5 — Isolation / enterprise | Planned |

## Development

```bash
make test
make check          # fmt + lint + test + security (requires golangci-lint, nox)
make cover          # coverage + coverctl ratchet
make install-hooks
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
