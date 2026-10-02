# Compatibility and the road to 1.0

Vitra follows [Semantic Versioning](https://semver.org). This page says what that covers today, what it will cover from 1.0, and what has to be settled before 1.0.

## Before 1.0 (now)

- **Patch releases** (`0.7.0` → `0.7.1`) never break code or behaviour, except to close a security hole. Those changes are listed under *Security* in the [changelog](https://github.com/klarlabs-studio/vitra/blob/main/CHANGELOG.md).
- **Minor releases** (`0.7` → `0.8`) may break things. Every breaking change is marked in the changelog, and each release has an *Upgrading* section with what to change.
- Pin a minor version (`go get go.klarlabs.de/vitra@v0.7`) if you need stability now.

## From 1.0

### What is covered

| Surface | Promise |
|---|---|
| Exported Go API of `vitra`, `app`, `domain`, `desktop`, `platform` and its OS packages, `plugin`, `plugin/official`, `audit`, `policy`, `updater`, `worker` | No breaking change within 1.x |
| Command names, permission names, event names, and denial codes | Stable within 1.x |
| Wire formats: the IPC envelope, signed update manifests, policy documents, audit event JSON | Versioned; a 1.x runtime reads every format version written by an earlier 1.x |
| The page API: `window.vitra.invoke` and `window.vitra.on` | Stable within 1.x |
| Generated TypeScript clients | Regenerating with a newer 1.x CLI never changes existing method signatures |
| CLI commands and flags, and `--json` output | Stable within 1.x |

### What is not covered

- Anything under `internal/`.
- Human-readable CLI output (use `--json` where it exists).
- Error message text. Match on errors with `errors.Is`/`errors.As` and on denial codes, never on strings.
- Exact audit `Detail` strings.
- Behaviour that turns out to be a security hole. Closing one may break code that relied on it. That is a patch release, listed under *Security*.

### Deprecation

1. A deprecated identifier gets a `// Deprecated:` comment naming its replacement, and a changelog entry.
2. It keeps working for at least two minor releases, and at least six months.
3. It is removed only in the next major version.

### Supported toolchains and platforms

- **Go:** the two most recent Go releases. The `go` line in `go.mod` moves forward only in minor releases.
- **macOS** 11 or later; **Windows** 10 and 11 with the WebView2 Evergreen Runtime; **Linux** with WebKitGTK 4.1 and GTK 3.
- Dropping a platform version is a minor-release change, announced one release ahead.

## Before 1.0: decisions still open

These are the API questions to settle while breaking changes are still cheap. Progress is tracked in the [Road to 1.0 issue](https://github.com/klarlabs-studio/vitra/issues/292).

1. **Typed official commands.** The official plugin commands decode loose JSON, and the generated client types them as `unknown`. Register them as typed commands (`vitra.Command`), so inputs are decoded strictly and the client is typed.
2. **Host capabilities in one place.** Optional host features are detected through interfaces scattered across packages: `SetDevTools`, `SetRejectHandler` and `SetMessageHandler` are unexported in `app`, while `MultiFileOpener` is exported in `platform`. Third-party hosts cannot discover the hidden ones. Export and document all of them in `platform`.
3. **A smaller host interface.** `app.DesktopHost` has about 35 methods. Split it into small interfaces (windows, dialogs, clipboard, menus, …) so a custom or test host implements only what it supports.
4. **The `desktop` input parsers.** Ten `desktop.Parse*` functions are exported only for binding the official commands. Once (1) lands, unexport them or move them behind the typed commands.
5. **The `domain` surface.** `domain` exports 72 identifiers, including pipeline internals such as the invocation service. Keep what apps use (grants, path scopes, callers, denials, typed invocations) and move the rest to `internal/`.
6. **Versioned wire formats.** Add a schema version to signed update manifests and policy documents, as the IPC envelope already has (`protocol: "1"`), so formats can evolve within 1.x.
7. **Signed releases.** Notarize the macOS CLI binaries and sign the Windows ones, so downloads do not trigger OS warnings. This needs the project's Apple Developer ID and a Windows code-signing certificate.
