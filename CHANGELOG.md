# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Upgrading
- Update manifests, policy documents, and audit event JSON now carry a format version, `"schema": "1"`. Existing files keep working: a manifest or policy document without `schema` is read as schema 1, and manifests signed by 0.9 still verify.
- Upgrade apps before you publish manifests signed by this CLI or deploy policies saved by this release. An older runtime reads the new files, but cannot tell a future format from this one; from this release on, a runtime refuses formats newer than it knows with `ErrUnsupportedSchema`.
- Add `"schema": "1"` to hand-written policy documents, and re-sign update manifests with this CLI (`vitra update-sign`). Unversioned manifests and policy documents are deprecated and stop being accepted before 1.0.
- If you parse audit JSON lines strictly, allow the new leading `schema` field. If you write audit JSON from a custom sink, encode with `audit.MarshalEvent`.
- Apps that pass `linux.New()`, `darwin.New()` or `windows.New()` to `app.Options.Host` need no change.
- If you call host methods beyond the core through an `app.DesktopHost` value (for example `host.SetTray`, `host.TrySingleInstance`, `host.Eval`), keep the concrete host type (`*linux.Host`, …), declare your own interface that embeds `app.DesktopHost` and the `platform` capabilities you use, or type-assert (`host.(platform.Tray)`).
- Custom host authors: implement `platform.DesktopHost` (12 methods) plus only the capabilities you support. Drop methods you only stubbed out; a missing capability now fails with `*platform.ErrUnsupported`. A host that shows menus, a tray or global shortcuts must also implement `SetActionHandler` (`platform.ActionReporter`). Add `var _ platform.X = (*YourHost)(nil)` for each interface you mean to implement, so a signature change fails to compile instead of silently dropping the capability.
- If you construct `desktop.MenuService` or `desktop.TrayService` yourself, set `OnSet` and `OnClear`; a nil hook now fails with `ErrUnsupported`.
- The official commands decode their input strictly. A page that sends an unknown field (`{"id": "main", "force": true}`), a wrong type (`"multiple": "true"`, `"width": "640"`), a non-integer or negative window size, or a menu item with `children` now gets a validation error (code `error`) where the extra data used to be ignored. Send only the fields listed in the [plugin reference](https://klarlabs-studio.github.io/vitra/reference/plugins#inputs-and-outputs). Command names, permissions, the documented shorthands (a bare window id, a bare menu array, ...), and outputs are unchanged; commands that return nothing still return `null`.
- Regenerate your TypeScript client (`vitra generate typescript --app .`). Official commands are now typed instead of `unknown`, so casts such as `(await client.dialogOpen(opts)) as string[] | null` can go, and TypeScript now reports calls that pass the wrong shape.
- `desktop.ParseMenuItems`, `ParseTraySet`, `ParseDialogFileOptions`, `ParseShortcutRegister`, `ParseShortcutUnregister`, `ParseDragDropEnable`, `ParseWindowCreateOptions`, `ParseWindowID`, `ParseWindowAlwaysOnTop`, `ParseWindowSetTitle`, `ParseWindowSetSize`, `ParseWindowSetIcon`, and `ParseWindowChromeApply` are removed. If you bound official commands by hand with them, bind with `vitra.Bind` and the matching input type from `plugin/official` instead (for example `vitra.Bind(rt, "window.setSize", func(ctx context.Context, inv domain.Invocation, in official.WindowSizeInput) (vitra.Void, error) { ... })`), passing `inv.Caller` to the `desktop` service as before.
- If you called official commands through `Runtime.Invoke` in Go, `window.create` now returns `official.WindowCreated` instead of `map[string]any` (same JSON).

### Added
- Linux: global shortcuts on Wayland through the `org.freedesktop.portal.GlobalShortcuts` portal (GDBus, no new dependency). `Features()` reports `shortcut.global` available on Wayland only while the portal is running; without it `RegisterGlobalShortcut` still returns `ErrUnsupported`. The desktop may ask the user to approve each binding, and a refusal is returned as an error. Activations arrive as `shortcut.action` events as on X11. Unregistering rebinds the remaining shortcuts on a new portal session, since the portal cannot unbind one. Accelerators are passed as the portal's preferred trigger (`Ctrl+Shift+Y` → `CTRL+SHIFT+y`). (#275)
- `schema` format version on signed update manifests (`updater.ManifestSchema`), covered by the signature: stripping, adding, or changing it invalidates the manifest. `SignManifest` and `BuildSignedManifest` always write it.
- `updater.ParseManifest`, which decodes a manifest and checks its schema. `Fetcher.FetchManifest`, `vitra update-apply`, and `vitra update-stage` use it.
- `schema` format version on policy documents (`policy.DocumentSchema`). `Document.Encode` and `Save` always write it; `ParseDocument` checks it before any other field.
- `schema` format version on audit event JSON (`audit.EventSchema`), written by `JSONLSink` and `FileSink`. New `audit.MarshalEvent` and `audit.ParseEvent` encode and decode one line.
- `updater.ErrUnsupportedSchema`, `policy.ErrUnsupportedSchema`, and `audit.ErrUnsupportedSchema` for a format version this runtime does not know. Match them with `errors.Is`.
- The release pipeline can notarize the macOS `vitra` binaries and Authenticode-sign the Windows ones. It turns on once the signing certificates are added as repository secrets and is skipped until then; checksums, SBOMs, and the cosign signature cover the signed binaries. See *Code-signing certificates* in `CONTRIBUTING.md`.
- Every host interface is exported and documented in package `platform`: the core `DesktopHost`, the optional `Clipboard`, `Dialogs`, `MultiFileOpener`, `Notifier`, `MenuBar`, `Tray`, `GlobalShortcuts`, `ActionReporter`, `DragDrop`, `WindowControls`, `URLOpener`, `PathOpener`, `SingleInstance`, `URLSchemeRegistrar`, `FileAssociationRegistrar`, `ScriptEvaluator` and `FileDropInjector`, and the hooks `DevToolsSetter`, `MessageReporter` and `RejectReporter` (previously unexported in `app`). See the new [Host interfaces](https://klarlabs-studio.github.io/vitra/reference/hosts) reference.
- `vitra.Bind` attaches a typed handler to a command a plugin contributed, keeping its name, description, and permission: input is decoded strictly and `Runtime.TypeScript` types it, as for `vitra.Register`.
- `vitra.Void`, the output type of a command that returns nothing. The page receives `null`; the generated client types the call as `Promise<void>`.
- Input and output types for every official command in `plugin/official` (`WriteFileInput`, `DialogOptions`, `FileFilter`, `MessageDialogInput`, `NotificationInput`, `WindowRef`, `WindowCreateInput`, `WindowCreated`, `WindowChromeInput`, `WindowAlwaysOnTopInput`, `WindowTitleInput`, `WindowSizeInput`, `WindowIconInput`, `MenuInput`, `MenuItem`, `TrayInput`, `DragDropInput`, `ShortcutInput`, `ShortcutRef`), with fuzz tests.
- The generated client marks an input optional when all its fields are optional (`dialogOpen(input?: DialogOptions)`).
- Linux: the tray is a StatusNotifierItem with a `com.canonical.dbusmenu` menu, spoken over GDBus with no new dependency (no libayatana-appindicator), whenever a `org.kde.StatusNotifierWatcher` is on the session bus. It shows on KDE Plasma, GNOME with the AppIndicator extension, and most other panels, on X11 and on Wayland. Without a watcher it falls back to `GtkStatusIcon` as before. Menu clicks and left clicks still arrive as `tray.action` (item ID / `tray.activate`), and `Features()` reports which protocol is in use.

### Changed
- **Breaking:** `app.DesktopHost` is now an alias of `platform.DesktopHost`, the 12-method core `app.Run` needs, instead of a 40-method interface. Everything else is an optional `platform` interface the app detects on the host; an official command whose capability is missing fails with `*platform.ErrUnsupported` naming the feature.
- **Breaking:** `desktop.MenuService` and `desktop.TrayService` with no `OnSet`/`OnClear` hook fail with `*platform.ErrUnsupported` instead of returning success without doing anything, like the other services.
- `vitra register-scheme` and `vitra register-files` report a host without the capability as `*platform.ErrUnsupported`.
- **Breaking:** `app.UseOfficialPlugins` binds every official command as a typed command. Input is decoded strictly: unknown fields, wrong types, and invalid window sizes are rejected instead of ignored. Shorthand payloads and every security check (grants, path scopes, deny rules, caller identity, TOCTOU-safe file access) are unchanged. Part of [#292](https://github.com/klarlabs-studio/vitra/issues/292).
- **Breaking:** `window.create` and `window.chrome` reject a width or height that is negative, fractional, or above 2^53 instead of silently using the default size.
- `vitra generate typescript` (with or without `--app`) and the clients `vitra new` writes now type every official command instead of `unknown`.

### Deprecated
- Update manifests and policy documents without a `schema` field. They are read as schema 1 for now and stop being accepted before 1.0.

### Removed
- **Breaking:** the `desktop.Parse*` input parsers. Their validation moved into the official input types in `plugin/official`, behind the typed commands.

### Fixed
- macOS: a host call made from another goroutine before `Run` started the loop (for example opening a dialog or a window from a startup goroutine) ran AppKit code off the main thread, and AppKit aborted the process. Such calls are now queued and run on the main thread once the loop starts.
- Linux and Windows: the same call before `Run` ran inline on whatever thread made it. On Linux that drove GTK, which is not thread-safe, from the wrong thread; on Windows a window created there belonged to that thread and `Run`'s message loop never pumped it. Before `Run`, a call now runs inline only on the main thread, where `Run` must be called; from any other thread it is queued and runs on the main thread once the loop starts.

## [0.9.0] - 2026-10-03

Consistent errors from every stub host, and a complete Go reference on
pkg.go.dev.

### Upgrading from 0.8.0
- Only affects builds without `-tags vitra_native` on Linux. The stub host's errors are now `*platform.ErrUnsupported`. If you compared error text, use `errors.As` and check `Feature` instead.

### Changed
- **Breaking:** the Linux stub host (built without `-tags vitra_native`) now fails with `*platform.ErrUnsupported` naming the missing feature, like the macOS and Windows stubs, instead of a plain error. Code that matched the old message text should use `errors.As` instead.
- The stub hosts that pkg.go.dev renders now document every method, and `platform/linux` has a package overview there.

## [0.8.0] - 2026-10-02

Deny rules that hold across folder aliases and Unicode spellings, iframes
blocked on Windows, the notes demo tested end to end on all three
platforms, and the first contributor issues shipped: `doctor --json`, a
rotating audit sink, `vitra new --with`, and multi-select file dialogs.

### Upgrading from 0.7.1
- No code changes required.
- Windows: remote pages in `<iframe>`s are now blocked by the navigation policy, as on Linux and macOS. Serve every frame from the app's own assets.
- Window sizes from the page above 2^53 or outside the `int` range are now rejected instead of rounded or clamped.

### Security
- Windows: the navigation policy now applies to iframes. WebView2's `NavigationStarting` fires for the top-level document only, so a remote page embedded in an `<iframe>` loaded on Windows (it still could not call Go: it has no sender token and WebView2 does not deliver its messages to the app). Frame navigations now go through the same check and are blocked and audited as `navigation.block`, as on Linux and macOS. Found by the new Windows E2E run of the notes demo.
- Deny patterns cover every spelling of their folder. A deny written as `/var/app/secret/**` also refuses `/private/var/app/secret/...` (macOS `/var` is a link): `Runtime.RegisterGrant` adds the real spelling of each deny pattern's folder, and file operations check each file's resolved real path against the deny patterns, so a deny written with the real path also applies through an alias. Allow patterns are never widened. New `domain.PathScope.WithDenyAliases` and `CapabilityGrant.WithDenyAliases`.
- On macOS, a deny pattern also refuses other Unicode spellings of its folder name (composed vs decomposed `é`), which APFS treats as the same file. Names are compared in the spelling stored on disk, read from open handles; no Unicode tables are added to the kernel.

### Added
- `vitra doctor --json` prints every doctor check as one JSON object, each with a `name`, a `status` (`ok`, `warn`, `fail`), and a `detail`, for CI jobs and bug reports. The text report and the exit status are unchanged. The bug report template now asks for this output.
- `vitra new <dir> --with fs,dialog,clipboard,notification,os` scaffolds an app that registers exactly those official plugins with least-privilege grants: `fs.read` and `fs.write` scoped to one per-user data folder (symlinks resolved), `dialog.open` and `dialog.save` only, `clipboard.write` only (reading is a commented opt-in), `notifications.show`, `os.info`. `path.open` is never granted. The frontend (vanilla page, or `src/plugins.ts` for the Vite templates) makes one call per plugin, and the shipped `vitra-client.ts` matches `vitra generate typescript --app`. An unknown plugin name is an error listing the valid ones. Without `--with`, the output is unchanged.
- `audit.FileSink` (`audit.NewFileSink(path, maxBytes, maxBackups)`): writes audit events as JSON lines to a file, rotates it before it would exceed `maxBytes`, and keeps `maxBackups` old files (`audit.jsonl.1` newest). Files are mode `0600`, an existing file is reopened and appended to, and `Append` never waits for the disk: a full queue returns `audit.ErrSinkFull`. Dropped events are recorded in the file as an `audit.dropped` event (new `audit.KindAuditDropped`) with the number lost. Write errors surface from `Flush` and `Close`.
- `dialog.open` can select several files: pass `{"multiple": true}` and it returns every selected path. The Linux (GTK), macOS (`NSOpenPanel`), and Windows (`IFileOpenDialog`) hosts support it; without `multiple` the dialog is unchanged. `platform.DialogFileOptions` has a new `Multiple` field, and hosts opt in through the new optional `platform.MultiFileOpener` interface, so custom `app.DesktopHost` implementations keep compiling. A host without it returns `platform.ErrUnsupported` instead of returning one file. Picking files grants nothing: `fs.read` still needs its own grant.

### Fixed
- Window sizes sent by the page (`window.setSize`, `window.create`, `window.chrome`) are accepted only as the exact integer the page sent: values above 2^53, outside the `int` range, or non-integral are rejected. Before, conversion was CPU-dependent (arm64 clamped, amd64 rejected) and large values were rounded. Found by the new fuzz targets for every `desktop.Parse*` input parser.
- Linux: the native host builds without warnings. The tray stays on `GtkStatusIcon`, GTK3's only built-in tray API, with its deprecation warnings silenced in that code only.

## [0.7.1] - 2026-10-02

Security fix: file access through `desktop.FileService` and `path.open` can no longer be redirected by a directory swap after the path was checked. No API changes.

### Security
- File reads and writes through `desktop.FileService` can no longer be raced. Before, the path was checked and then opened again, so a local process that swapped a directory in between could redirect the operation into a denied folder or out of the scope. Files are now opened through an `os.Root` at the scope's real root, the opened file's actual location is looked up from the handle and authorized again, and new files are created exclusively inside the authorized directory's handle. `path.open` is verified the same way, and hands the OS opener the file's real, symlink-free location only after checking it still leads to the verified file. The opener's own open of that path is the one remaining window.

## [0.7.0] - 2026-09-30

Every bridge message is checked against the document that sent it, forged
and tampered updates have their own errors, and pkg.go.dev has runnable
examples.

### Upgrading from 0.6.0
- `about:blank` documents can no longer call commands. Serve every page that calls Go from the app's assets.
- Custom `app.DesktopHost` implementations keep working; implement `SetMessageHandler` to get the per-message sender check.
- To tell a forged or tampered update apart, use `errors.Is(err, updater.ErrBadSignature)` / `updater.ErrDigestMismatch` instead of matching message text.

### Security
- The origin of a call is now checked on every bridge message. All three native hosts report the document that sent each message, and the app accepts it only from its own asset server, deriving the call's origin from it. Messages from any other document (a remote page, `about:blank`, another local port) are dropped and audited as `bridge.reject`. This closes the last documented bridge limit; before, the origin was the one last recorded for the window and relied on the navigation policy. Hosts gain an optional `SetMessageHandler`.

### Added
- `updater.ErrBadSignature` and `updater.ErrDigestMismatch`, so callers can recognise a forged or tampered update with `errors.Is` instead of matching message text.
- Runnable examples on pkg.go.dev for typed commands, the TypeScript client, auditing, path scopes, grants, the update check, enterprise policy, and `App.UseOfficialPlugins`.

## [0.6.0] - 2026-09-30

One call to turn on the official desktop plugins, a documentation site, and
deterministic refusals for unregistered commands.

### Upgrading from 0.5.0
- No code changes required. Apps that bind the official plugins by hand can replace that code with `a.UseOfficialPlugins(ctx)`; grants stay as they are.
- Calls to unregistered commands now fail with the `command_missing` denial code instead of `error`.

### Added
- `App.UseOfficialPlugins(ctx, plugins...)` registers the official plugins (all of them, or the ones passed) and binds every command to the native host through the capability-checked `desktop` services, replacing about 40 hand-written `BindExecutor` calls. It grants nothing. It also forwards native menu, tray, and shortcut activations and file drops as events. `example/competitive` uses it (987 → 580 lines).

### Fixed
- Calling a command that was never registered is refused with the `command_missing` denial code (and audited as `denied`) instead of a generic error. The code existed but was never returned.

## [0.5.0] - 2026-09-30

A flagship demo that attacks itself, a hello-world `vitra new`, audit events
for refusals that happen before a command runs, and a warning-free macOS build.

### Upgrading from 0.4.0
- Existing apps need no code changes.
- macOS: the native host requires macOS 11 or later.
- macOS: `ShowNotification` from an unbundled binary (`go run`, `vitra dev`) now returns `platform.ErrUnsupported`; package the app to show notifications.
- `vitra new` no longer wires the official plugins; copy that wiring from `example/competitive` if you relied on it.

### Added
- `example/notes` (`make notes`): a Markdown notes app over a vault folder with a live audit panel and a "Try to break it" panel that runs nine real attacks (path escapes, a denied folder, writing a script, a path-binding mismatch, remote navigation and frames, a forged bridge call). CI drives its UI in WebKitGTK (`make e2e-notes`).
- Audit events for attacks the app stops before a command runs: `bridge.reject` (a message without the window's sender token, an unparseable message, an invalid top-frame message, or, on macOS, a message from a subframe) and `navigation.block` (a navigation the policy refused, logged without credentials, query, or fragment).
- `command.invoke` audit events carry the checked `resource_path` and, for denials, the denial `code` in `Metadata`, so the log says what was attempted and why it was refused.

### Changed
- `vitra new` generates a 120-line hello-world instead of a 658-line tour of every plugin: one typed `greet` command, one grant, and a matching generated client. Every template's frontend calls `client.greet`. The plugin tour lives on in `example/competitive`, and `example/notes` shows a complete app.

### Fixed
- macOS: the native host builds without deprecation warnings. Drag and drop reads file URLs (`NSPasteboardTypeFileURL`), dialog filters use `UTType`, and notifications use `UNUserNotificationCenter`. Requires macOS 11 or later.
- macOS: notifications from an unbundled binary (`go run`, `vitra dev`) return `platform.ErrUnsupported` and `FeatureNotificationShow` reports unavailable, instead of silently showing nothing. Package the app (`vitra package --format app-dir`) to show them.

## [0.4.0] - 2026-09-30

Security hardening from an external-style audit, typed commands with generated
TypeScript clients, and a much smaller public surface. Pre-1.0: several
breaking changes, marked below.

### Upgrading from 0.3.0
- Update code: `PlanInstall` / `Runtime.ApplyUpdate` take `updater.Installed`; re-sign update manifests (they now need `expires_at`).
- Imports: `plugin/official/<name>` → `plugin/official` (`official.FS()`, `official.All()`); packages moved under `internal/` are no longer importable.
- `vitra update-apply` needs `--app-id` and `--current-version`.
- Path scope patterns must be absolute.

### Added
- Release pipeline: pushing a `v*` tag publishes the `vitra` CLI for linux, darwin, and windows (amd64, arm64) with reproducible `-trimpath` builds, an SPDX SBOM per archive, a keyless cosign signature over the checksums, and SLSA build provenance. Release notes include verification commands.
- OpenSSF Scorecard runs weekly and on `main`; README badge.
- `updater.CompareVersions`: SemVer 2.0.0 precedence (prereleases, build metadata ignored).
- Typed commands: `vitra.Register(rt, vitra.Command[In, Out]{Name, Description, Permission, Handler})`. Input is decoded strictly into `In` (unknown fields and type mismatches are rejected as validation errors), the handler receives the authorized `domain.Invocation`, and when `In` implements `ResourcePath() string` it must equal the path the gateway checked.
- `Runtime.TypeScript(module)`: a TypeScript client for every registered command, with interfaces generated from typed commands' Go types (JSON tags, `omitempty`, pointers, slices, maps, embedded and recursive structs, `time.Time`, `[]byte`).
- `vitra generate typescript --app <dir>` runs the app in code-generation mode (`app.EnvGenerateTypeScript`) so the client covers the app's own commands. Works with the stub host; no cgo needed.
- The `vitra new` starter's `demo.greet` is a typed command, and its README uses `--app .`.
- `domain.InvocationFrom(ctx)`: executors can read the authorized `Invocation` (caller, command, checked `ResourcePath`, and matching grant) from their context.
- `domain.CallerExecutorFunc`: an executor adapter that receives the caller the gateway authorized. It fails with `ErrNoInvocation` when called outside the gateway.
- `domain.Decision.ScopeRoot`: for an allowed path-scoped permission, the literal root of the allow pattern that matched (`/project` for `/project/**`).

### Changed
- **Breaking:** the 14 official plugin packages (`plugin/official/fs`, `…/dialog`, …) are one package, `plugin/official`: constructors `official.FS()`, `official.Dialog()`, … `official.DeepLink()`, ids `official.FSID`, …, and `official.All()` for every plugin. Each old package held one constructor and one constant. The `vitra new` starter registers them with a single loop over `official.All()`.
- **Breaking:** `updater.PlanInstall(m, pub, artifact, installed)` and `Runtime.ApplyUpdate(m, pub, artifact, dest, installed)` require the installed app description. `vitra update-apply` requires `--app-id` and `--current-version` in both local and channel mode.
- **Breaking:** `bindings.GenerateTypeScript` takes `[]bindings.Command` (use `bindings.Untyped(defs...)` for definitions without types) and returns an error.
- Starter templates use React 19, TypeScript 7, Vite 8, `@sveltejs/vite-plugin-svelte` 7, and `vue-tsc` 3 (via Dependabot). CI now scaffolds each Vite template and runs `npm install`, `npm run build`, and `go build` on the result, so dependency bumps cannot break `vitra new` unnoticed.
- CI pins GitHub Actions to commit SHAs.
- `PathScope` patterns must be absolute. `vitra inspect capabilities` shows `/project/**` instead of the unexpanded `${PROJECT_DIR}` placeholder.

### Removed
- **Breaking:** packages apps never need are no longer importable: `application`, `inmemory`, `ipc`, `bridge`, `bindings`, `packaging`, `provenance`, and `platform/null` moved under `internal/`. The public surface is `vitra`, `app`, `domain`, `desktop`, `platform/*` hosts, `plugin`, `policy`, `audit`, `updater`, and `worker`. `Runtime.EmitEvent` now returns `[]domain.EventDelivery` (was `application.EventDelivery`).
- **Breaking:** `vitra new` supports five starters: `vanilla`, `vite`, `react`, `svelte`, `vue`. The other 47 templates (solid, preact, lit, alpine, htmx, angular, qwik, … uland) are gone; start from `vite` and build to `frontend/dist` for any other framework. Each template pinned npm versions that went stale without anyone noticing, and several targeted abandoned projects. Starter files now live under `cmd/vitra/templates/` as real files embedded with `embed.FS`, instead of ~6,000 lines of Go string literals; `cmd/vitra/main.go` shrinks from 9,000 to 1,500 lines. Output of the five kept templates is unchanged apart from the vite README heading.

### Fixed
- Updates install correctly beyond a single Linux binary. `updater.ApplyInstall` (and `Runtime.ApplyUpdate` / `vitra update-apply`) now:
  - replaces a whole directory, such as a macOS `.app` bundle, from a `.tar.gz`/`.tgz`/`.zip` artifact (a single top-level directory in the archive becomes the destination), keeping exec bits and internal symlinks;
  - replaces a running Windows executable by moving it aside first, since Windows can rename a running `.exe` but not overwrite it;
  - writes the new version fully before replacing anything and restores the old one if the swap fails;
  - rejects archive entries that are absolute, use `..`, or are symlinks pointing outside the archive, and archives over 2 GiB unpacked or 200,000 entries.

  `updater.CleanupStale(dest)` removes replaced installs that could not be deleted while running.
- Windows `register-scheme` / `register-files`: the generated `.reg` files left the quotes around the executable path and `"%1"` unescaped, producing an invalid command value; quotes are now escaped and CR/LF stripped from values.
- Windows: host calls from command handlers run on the UI thread. `vitra_idle_add` ran queued jobs immediately on the calling thread, so since invokes moved off the UI thread, clipboard, dialog, and window calls touched Win32/WebView2 objects from goroutines. Jobs are now posted to a message-only window on the UI thread (which keeps working during modal dialogs), and run inline when already on it.
- Native hosts pass job ids to C as integers instead of casting them through `unsafe.Pointer` (`go vet` warning).
- CI builds and unit-tests the Darwin and Windows native hosts (`-tags vitra_native`) on macOS and Windows runners; before, only Linux native code was compiled in CI.
- Native hosts (Linux, Darwin, Windows) run frontend invokes, menu/tray/shortcut actions, and file-drop handlers off the UI thread. Before, a command that called back into the host (clipboard, dialogs, `Eval`, `App.Emit`), or an action handler that emitted an event, queued work to the UI thread from the UI thread and waited: the app froze. Slow commands no longer block the UI either. Replies are posted back to the UI thread and dropped if the loop has stopped.
- `make e2e` round-trips `clipboard.write` → `clipboard.read` → `demo.greet` and is bounded by `timeout`, so a hang fails instead of stalling CI.
- Generated TypeScript: two commands mapping to the same method (`fs.read` / `fs_read`) is an error instead of a duplicate method; descriptions can no longer close the doc comment (`*/`); string literals are valid JavaScript (JSON-encoded instead of Go `%q`, which could emit `\U` escapes); non-identifier names are quoted.
- `vitra new` writes a `go.mod` that builds as generated: `go 1.26.2` (was `go 1.26`, older than vitra's, forcing `go mod tidy`) and `require go.klarlabs.de/vitra v0.3.0` (was the unresolvable `v0.0.0`).

### Security
- Update manifests carry a signed `expires_at`; `PlanInstall` refuses manifests that are expired or have none, so a mirror cannot keep serving an old signed release forever (freeze attack). `BuildSignedManifest` defaults to 90 days (`DefaultManifestTTL`); `BuildSignedManifestTTL` and `vitra update-sign --expires-in 30d|72h` set it explicitly. **Breaking:** manifests signed before this change must be re-signed.
- Only a window's top frame can invoke commands. Each window gets a random 256-bit sender token, kept in the preload's closure (which returns early in subframes) and required on every bridge message; messages without it, or unparseable ones, are dropped without a reply. Before, any frame that could reach the native message handler, such as a cross-origin iframe (WebView2 injects the preload into every frame), was stamped with the window's identity. macOS additionally drops messages whose `frameInfo` is not the main frame.
- Inbound invoke messages over 1 MiB (`ipc.MaxMessageBytes`) are rejected before JSON parsing; the decoder is fuzzed for panics and for identity always coming from the host.
- The WebView inspector is off by default on all hosts. Linux forced WebKitGTK developer extras on and Windows left WebView2 DevTools at its enabled default, letting anyone at the keyboard run script with the page's bridge access. Opt in with `app.Options.DevTools`; `vitra dev` enables it via `VITRA_DEVTOOLS=1`.
- `vitra new` scaffolds least privilege. The main window no longer gets `fs.read`, `fs.write`, `path.open`, `browser.open`, or `clipboard.read` by default; they are listed commented out with guidance. The `aux` window only gets `demo.greet` instead of every permission. Before, the default grant let the frontend write a script with `fs.write` and launch it with `path.open`. Grant errors are no longer ignored.
- `example/competitive` denies `path.open` inside its writable demo directory, under both its literal and symlink-resolved names.
- Updates refuse validly signed releases that are not an upgrade. `updater.PlanInstall` and `Runtime.ApplyUpdate` take an `updater.Installed{AppID, Channel, Version}` and return `ErrWrongApp`, `ErrWrongChannel`, or `ErrNotNewer` unless the manifest is for the same app and channel and has a strictly newer SemVer version. Before, any old signed manifest could be replayed (downgrade), a beta build installed on stable, and another app's release installed when keys were shared.
- Update channels must use `https`; plain `http` is accepted only for loopback hosts.
- The `vitra new` scaffold and `example/competitive` executors act as the window that invoked them. Before, they re-authorized every desktop service call as a hard-coded `main` caller, so a secondary window used `main`'s grants. Host-initiated work (startup menus, deep links, single-instance) uses an explicit `hostCaller`.
- Path scopes close several bypasses of `PathScope.Matches`:
  - deny patterns now match case-insensitively, so `/project/.SECRETS/key` no longer slips past a `/project/.secrets/**` deny on APFS/NTFS;
  - backslashes are treated as separators, so `..\..\` traversal is rejected;
  - relative, drive-relative, UNC (`\\host\share`), and device (`\\?\`) paths are rejected;
  - segments are now `path.Match` globs, so partial patterns such as `**/*.pem` actually match instead of silently never denying.
- `NewCapabilityGrant` rejects malformed or relative path patterns, and a deny pattern that cannot be compiled denies.
- `FileService` (`fs.read`/`fs.write`) and `PathService` (`path.open`) resolve symlinks before acting. The real target must stay under the real root of the allow pattern that matched, and its location is authorized again, so deny rules apply to what a link points at. Before, a link inside the scope (`proj/escape -> /etc`) read or wrote anywhere on disk, and `proj/public -> .secrets` bypassed a `.secrets/**` deny. Dangling links are refused for writes.
- The navigation allow-list compares the parsed scheme and host:port against the asset server exactly. The previous string-prefix check let `http://127.0.0.1:PORT@evil.example/` (userinfo) and `http://127.0.0.1:PORT1/` (another local port) keep privileged bridge access.
- Windows `OpenURL` no longer shells out through `cmd /c start`. cmd.exe interprets `&`, `|`, `^`, `<`, `>` even inside a quoted argv element, so a `browser.open` URL like `https://a.example/?x&calc` could run arbitrary commands. It now uses `rundll32 url.dll,FileProtocolHandler`, which never parses shell metacharacters.

## [0.3.0] - 2026-09-28

First tagged release. The secure runtime kernel is complete and runnable
DesktopHost adapters ship for Linux (WebKitGTK), Darwin (WKWebView), and
Windows (WebView2) behind `-tags vitra_native`.

### Highlights
- **Secure kernel:** capability grants, gateway with deterministic denials,
  explicit commands, host-stamped caller identity, path scopes with deny-wins.
- **Native hosts:** menus, tray, multi-window, host→frontend events, file
  drag-drop, dialogs, deep links, single-instance, and global shortcuts on all
  three platforms (Linux global shortcuts are X11-only; Wayland is explicitly
  unsupported).
- **Distribution:** Linux `.deb`/`.rpm`/`.snap`/`.flatpak`/AppDir/AppImage,
  Windows `msi`/`nsis`/WiX, Darwin `.app`/`.dmg`, and signed update apply.
- **CLI:** `vitra new` scaffolds, `vitra dev`, packaging, and scheme/file
  association registration.

### Known limitations
- Plugin SDK (Phase 3) and isolation/enterprise (Phase 5) ship as contracts
  plus runtime wiring; APIs may change before 1.0.
- Pre-1.0: public Go APIs are not yet stable.

### Fixed
- `cmd/vitra` and `packaging` build again on Darwin and Windows hosts: the portable Linux staging code no longer lives in a `_linux.go`-suffixed file, and `platform/linux` / `platform/darwin` single-instance flock calls are split behind `unix` build constraints (non-unix builds return an explicit unsupported error). CI now cross-builds the CLI for darwin and windows.

### Added
- AppStream project_group: metainfo always emits `<project_group>Vitra</project_group>` so software centers can group Vitra-packaged apps.
- `vitra new --template uland`: Vite + µland (`uland`) + TypeScript starter (`Component` / `html` / `useState` hooks + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- FreeDesktop SingleMainWindow: staged `.desktop` files (linux-dir / deb / rpm / snap / flatpak / AppDir) emit `SingleMainWindow=true` for GNOME Shell single-window awareness.
- `vitra new --template uce`: Vite + µce (`uce`) + TypeScript starter (`define` / `html` micro custom elements + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream branding: metainfo always emits `<branding>` primary colors (`#eeeeee` light / `#111111` dark) matching the Vitra scaffold chrome for software centers.
- `vitra new --template heresy`: Vite + Heresy + TypeScript starter (`define` / `html` / `render` custom elements + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream stock icon: metainfo emits `<icon type="stock">…</icon>` from `sanitizeFileName(Spec.Name)` (matches FreeDesktop `.desktop` `Icon=` / staged hicolor key).
- `vitra new --template neverland`: Vite + Neverland + TypeScript starter (`neverland` / `html` / `useState` hooks + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream developer: metainfo emits `<developer id="…"><name>…</name></developer>` (id from AppID parent DNS; name from `EffectivePublisher`) alongside legacy `<developer_name>`.
- `vitra new --template lighterhtml`: Vite + lighterhtml + TypeScript starter (`html` / `render` tagged templates + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- FreeDesktop StartupNotify: staged `.desktop` files (linux-dir / deb / rpm / snap / flatpak / AppDir) emit `StartupNotify=true` (and `Terminal=false` on linux-dir for parity).
- `vitra new --template dio`: Vite + Dio (`dio.js`) + TypeScript starter (`createElement` / `Component` / `render` hyperscript + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream pkgname: metainfo emits `<pkgname>` from `debianName(Spec.AppID, …)` (matches Debian `Package:` / RPM `Name:`).
- `vitra new --template crank`: Vite + Crank (`@b9g/crank`) + TypeScript starter (`html` tagged templates / generator components + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- FreeDesktop GNOME notifications: staged `.desktop` files (linux-dir / deb / rpm / snap / flatpak / AppDir) emit `X-GNOME-UsesNotifications=true` for GNOME Shell notification awareness.
- `vitra new --template fast`: Vite + FAST Element (`@microsoft/fast-element`) + TypeScript starter (`FASTElement` / `html` / `css` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream supports: metainfo always emits `<supports><control>touch</control></supports>` (touch input capability for software centers).
- `vitra new --template solid-element`: Vite + solid-element + Solid + TypeScript starter (`customElement` / `createSignal` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Darwin CFBundleSpokenName: `Spec.Name` flows into Info.plist `CFBundleSpokenName` (alongside `CFBundleName` / `CFBundleDisplayName`) for VoiceOver spoken app name.
- `vitra new --template moon`: Vite + Moon + TypeScript starter (`Moon` reactive UI + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream requires: metainfo always emits `<requires><display_length compare="ge">360</display_length></requires>` (desktop minimum display size for software centers).
- `vitra new --template ractive`: Vite + Ractive + TypeScript starter (`Ractive` / mustache templates + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream recommends: metainfo always emits `<recommends>` with `<control>keyboard</control>` and `<control>pointing</control>` (desktop input defaults for software centers).
- `vitra new --template htm`: Vite + htm + TypeScript starter (`html` tagged templates + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream provides: metainfo emits `<provides><binary>…</binary></provides>` from `sanitizeFileName(Spec.Name)` (matches staged Linux binary basename).
- `vitra new --template omi`: Vite + Omi + TypeScript starter (`define` / `WeElement` / `html` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Darwin NSSupportsAutomaticGraphicsSwitching: Info.plist always sets `NSSupportsAutomaticGraphicsSwitching` true (alongside `NSHighResolutionCapable`) for automatic GPU switching on dual-GPU Macs.
- `vitra new --template uhtml`: Vite + µhtml (`uhtml`) + TypeScript starter (`html` / `render` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream releases: metainfo emits `<releases><release version="…"/></releases>` from `Spec.Version` (omitted when empty).
- `vitra new --template hybrids`: Vite + Hybrids + TypeScript starter (`define` / `html` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream OARS content rating: metainfo always emits `<content_rating type="oars-1.1"/>` (empty = all attributes none; Flathub-friendly default for desktop apps).
- Official window unfullscreen: `window.unfullscreen` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Unfullscreen` (clears `Fullscreen`); scaffold/competitive bind.
- `vitra new --template arrow`: Vite + ArrowJS (`@arrow-js/core`) + TypeScript starter (`reactive` / `html` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream translate URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="translate">` alongside homepage/help/bugtracker/vcs-browser/donation/contact/faq/contribute (omitted when empty).
- `vitra new --template haunted`: Vite + Haunted + TypeScript starter (`component` / `html` / `useState` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Official window unmaximize: `window.unmaximize` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Unmaximize` (clears `Maximized`); scaffold/competitive bind.
- AppStream contribute URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="contribute">` alongside homepage/help/bugtracker/vcs-browser/donation/contact/faq (omitted when empty).
- `vitra new --template sinuous`: Vite + Sinuous + TypeScript starter (`h` / `observable` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream faq URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="faq">` alongside homepage/help/bugtracker/vcs-browser/donation/contact (omitted when empty).
- `vitra new --template atomico`: Vite + Atomico (`atomico`) + TypeScript starter (`c`/`html`/`useState` web component + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream contact URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="contact">` alongside homepage/help/bugtracker/vcs-browser/donation (omitted when empty).
- `vitra new --template reef`: Vite + Reef (`reefjs`) + TypeScript starter (`signal` / `component` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Official window setIcon: `window.setIcon` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.SetIcon` + `ParseWindowSetIcon` (`{ id, iconPath }`); scaffold/competitive bind.
- Windows ARP EstimatedSize: NSIS writes `EstimatedSize` (KB) and WiX sets `ARPSIZE` from staged binary (+ optional icon) size, matching Debian Installed-Size rounding.
- `vitra new --template vanjs`: Vite + VanJS (`vanjs-core`) + TypeScript starter (`van.tags` / `van.state` / `van.add` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Darwin CFBundleDisplayName: `Spec.Name` flows into Info.plist `CFBundleDisplayName` (alongside `CFBundleName`) for Finder/Dock display.
- Official window setTitle/setSize: `window.setTitle` / `window.setSize` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.SetTitle` / `SetSize` + `ParseWindowSetTitle` / `ParseWindowSetSize` (`{ id, title }` / `{ id, width, height }`); scaffold/competitive bind.
- Windows ARP NoModify/NoRepair: NSIS writes `NoModify`/`NoRepair` DWORD 1 under the Uninstall key; WiX sets `ARPNOMODIFY`/`ARPNOREPAIR` (hides Modify/Repair in Apps & Features for per-user installs).
- AppStream donation URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="donation">` alongside homepage/help/bugtracker/vcs-browser (omitted when empty).
- `vitra new --template cherry`: Vite + Cherry (ClojureScript dialect, `cherry-cljs/vite`) starter (`main.cljs` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template squint`: Vite + Squint (ClojureScript dialect, `squint-cljs/vite`) starter (`main.cljs` + vitra-client interop) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream vcs-browser URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="vcs-browser">` alongside homepage/help/bugtracker (omitted when empty).
- Official window restore: `window.restore` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Restore` (clears `Minimized`/`Maximized`/`Fullscreen`); scaffold/competitive bind.
- `vitra new --template rescript`: Vite + ReScript 11 + React (`App.res` + `@jihchi/vite-plugin-rescript`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream bugtracker URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="bugtracker">` alongside homepage/help (omitted when empty).
- Official window always-on-top: `window.setAlwaysOnTop` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.SetAlwaysOnTop` + `ParseWindowAlwaysOnTop` (`{ id, alwaysOnTop }`); scaffold/competitive bind.
- `vitra new --template elm`: Vite + Elm 0.19 + TypeScript bridge (`Main.elm` ports + `vite-plugin-elm`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream help URL: `Spec.Homepage` / `vitra package --homepage` also flow into metainfo `<url type="help">` alongside `<url type="homepage">` (omitted when empty; mirrors Windows ARP HelpLink).
- Official window fullscreen: `window.fullscreen` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Fullscreen` (read chrome → set `Fullscreen` → `Apply`); scaffold/competitive bind.
- Official window minimize/maximize: `window.minimize` / `window.maximize` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Minimize` / `Maximize` (read chrome → set `Minimized`/`Maximized` → `Apply`); scaffold/competitive bind.
- `vitra new --template dojo`: Vite + Dojo Framework 8 + TypeScript starter (`WidgetBase` with `v`/`w` virtual DOM) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template polymer`: Vite + Polymer 3 + TypeScript starter (`vitra-app` PolymerElement, `[[out]]` / `on-click`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream update_contact: `Spec.Maintainer` / `vitra package --maintainer` (via `EffectiveMaintainer`) flow into metainfo `<update_contact>` (defaults to `DefaultMaintainer`, matching snap `contact:`).
- `vitra new --template nerv`: Vite + Nerv.js 1.5 + TypeScript starter (`App.tsx` class component, esbuild `jsxFactory: h`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Official window blur: `window.blur` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Blur` → `DesktopHost.BlurWindow` (GTK clear-focus + lower, NSWindow `resignKeyWindow`/`orderBack`, Win32 `HWND_BOTTOM`); scaffold/competitive bind.
- `vitra new --template backbone`: Vite + Backbone 1.6 + Underscore + TypeScript starter (`Backbone.View` with `data-action` events) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template knockout`: Vite + Knockout 3 + TypeScript starter (`applyBindings` view-model, `data-bind`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template hyperapp`: Vite + Hyperapp 2 + TypeScript starter (`h`/`text`/`app` with effectful actions) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Windows ARP URLUpdateInfo: `Spec.Homepage` / `vitra package --homepage` also flow into NSIS `URLUpdateInfo` and WiX `ARPURLUPDATEINFO` (alongside URLInfoAbout / HelpLink; omitted when empty).
- `vitra new --template petite-vue`: Vite + Petite-Vue 0.4 + TypeScript starter (`createApp` + `v-scope`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template stimulus`: Vite + Hotwired Stimulus 3 + TypeScript starter (`vitra` controller, `data-action` bindings) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Official window hide/show: `window.hide` / `window.show` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Hide` / `Show` (read chrome → set `Hidden` → `Apply`); scaffold/competitive bind.
- `vitra new --template aurelia`: Vite + Aurelia 2 + TypeScript starter (`my-app` custom element, `@aurelia/vite-plugin`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template marko`: Vite + Marko 5 + TypeScript starter (`App.marko` class component, `@marko/vite` with `linked: false`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- `vitra new --template ember`: Vite + Ember 7 + Embroider (`@embroider/vite`, Glimmer `vitra-app` component) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Windows ARP HelpLink: `Spec.Homepage` / `vitra package --homepage` also flow into NSIS `HelpLink` and WiX `ARPHELPLINK` (alongside URLInfoAbout; omitted when empty).
- Darwin CFBundleGetInfoString: `Spec.Description` / `vitra package --description` (via `EffectiveDescription`) flow into Info.plist `CFBundleGetInfoString` (default: secure Go + web desktop blurb).
- `vitra new --template stencil`: Vite + Stencil 4 + TypeScript starter (`vitra-app` component, `@stencil-community/unplugin-stencil`, `dist-custom-elements`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Darwin LSApplicationCategoryType: `Spec.Categories` / `vitra package --categories` (via `LSApplicationCategoryType`) set Info.plist `LSApplicationCategoryType` (e.g. Development→public.app-category.developer-tools; default Utility→public.app-category.utilities).
- `vitra new --template inferno`: Vite + Inferno 9 + TypeScript starter (`App.tsx` class component, `vite-plugin-babel` + `babel-plugin-inferno`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Darwin Info.plist copyright: `Spec.License` / `vitra package --license` (via `EffectiveLicense`) flow into `NSHumanReadableCopyright` (default: `LicenseRef-proprietary`).
- Directory dialog options: optional `title` and `defaultPath` on `dialog.openDirectory` (same payload shape as open/save; filters ignored); native hosts apply them (GTK folder chooser, NSOpenPanel, IFileOpenDialog SetTitle/SetFolder); empty payload keeps prior defaults.
- Official window focus: `window.focus` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Focus` → `DesktopHost.FocusWindow` (GTK `present`, NSWindow `makeKeyAndOrderFront`, Win32 `SetForegroundWindow`); scaffold/competitive bind.
- Windows ARP LegalCopyright: `Spec.License` / `vitra package --license` (via `EffectiveLicense`) flow into NSIS `LegalCopyright` and WiX `ARPCOPYRIGHT` (default: `LicenseRef-proprietary`).
- Official menu clear: `menu.clear` on `vitra.menu` (same `menu.set` grant) via `desktop.MenuService.ClearMenu` → `DesktopHost.SetMenuBar(nil)`; scaffold/competitive bind.
- RPM Group: `Spec.Categories` / `vitra package --categories` (via `RPMGroup`) set `.spec` `Group:` (e.g. Development→Development/Tools; default Utility→Applications/System).
- Debian Section: `Spec.Categories` / `vitra package --categories` (via `DebianSection`) set control `Section:` (e.g. Development→devel; default Utility→utils).
- Official `vitra.deeplink` plugin: owns `deeplink.handle` and contributes `deeplink.open`; scaffold/competitive emit after `DeepLinkService.Handle` (host patterns; bridge + argv); `VITRA_INJECT_DEEPLINK=1` demo inject.
- Snap contact: `Spec.Maintainer` / `vitra package --maintainer` (via `EffectiveMaintainer`) flow into `meta/snap.yaml` `contact:` (defaults to `DefaultMaintainer`).
- `vitra new --template riot`: Vite + Riot.js 9 + TypeScript starter (`app.riot` + `rollup-plugin-riot`, `riot.component` mount) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Debian copyright: `Spec.License` / `vitra package --license` (via `EffectiveLicense`) write DEP-5 `usr/share/doc/<pkg>/copyright` in `BuildDeb` (Upstream-Name/Contact, optional Source from Homepage).
- Windows ARP Comments: `Spec.Description` / `vitra package --description` (via `EffectiveDescription`) flow into NSIS `Comments` and WiX `ARPCOMMENTS` / Package Description+Comments.
- Windows ARP support URL: `Spec.Homepage` / `vitra package --homepage` flow into NSIS `URLInfoAbout` / `HelpLink` and WiX `ARPURLINFOABOUT` / `ARPHELPLINK` (omitted when empty).
- Official shortcut unregister: `shortcut.unregister` on `vitra.shortcut` (same `shortcut.register` grant) via `desktop.ShortcutService.Unregister` → `DesktopHost.UnregisterGlobalShortcut`; scaffold/competitive bind (Wayland remains `ErrUnsupported`).
- Official tray clear: `tray.clear` on `vitra.tray` (same `tray.set` grant) via `desktop.TrayService.ClearTray` → `DesktopHost.ClearTray`; scaffold/competitive bind.
- Official window chrome read: `window.getChrome` on `vitra.window` (same `window.chrome` grant) via `desktop.WindowService.Read` → `DesktopHost.ReadWindowChrome`; scaffold/competitive bind.
- `vitra new --template mithril`: Vite + Mithril 2 + TypeScript starter (`mithril` hyperscript `m.Component`, `m.mount`) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Package keywords: `Spec.Keywords` / `vitra package --keywords` flow into FreeDesktop `.desktop` `Keywords=`, AppStream `<keyword>`, and snap `keywords:` (omitted when empty).
- Official `vitra.app` plugin: declares `app.quit`; `desktop.AppService` + scaffold/generate/provenance/competitive bind → `App.Quit`.
- Official `vitra.shortcut` plugin: declares `shortcut.register` / `shortcut.unregister` + `shortcut.action` event; `desktop.ParseShortcutRegister` / `ParseShortcutUnregister` + scaffold/generate/provenance/competitive bind `ShortcutService` → `RegisterGlobalShortcut` / `UnregisterGlobalShortcut` (Wayland remains `ErrUnsupported`).
- Official `vitra.dragdrop` plugin: declares `dragdrop.receive` + `dragdrop.drop` event; `desktop.ParseDragDropEnable` + scaffold/generate/provenance/competitive bind `DragDropService` → `EnableDragDrop`.
- Official `vitra.tray` plugin: declares `tray.set` / `tray.clear` + `tray.action` event; `desktop.ParseTraySet` + scaffold/generate/provenance/competitive bind `TrayService` → `SetTray` / `ClearTray`; native actions emit `tray.action` (alongside `menu.action`).
- Official `vitra.menu` plugin: declares `menu.set` / `menu.clear` + `menu.action` event; `desktop.ParseMenuItems` + scaffold/generate/provenance/competitive bind `MenuService` → `SetMenuBar` / clear; native actions emit `menu.action` (Quit still handled host-side).
- Official `vitra.window` plugin: declares `window.create` / `window.close` / `window.chrome` / `window.getChrome`; `desktop.WindowService` Create/Close/Apply/Read wrap `app.App` + `DesktopHost` chrome APIs; scaffold/generate/provenance/competitive bind (grant includes `aux`).
- Dialog open/save options: optional `title`, `defaultPath`, and `filters` (`[{ name, extensions }]`) on `dialog.open` / `dialog.save`; native hosts apply them (GTK filters, NSOpen/SavePanel allowedFileTypes, Win32 OFN filter); empty payload keeps prior unfiltered behavior.
- `vitra new --template qwik`: Vite + Qwik CSR + TypeScript starter (`@builder.io/qwik`, `qwikVite({ csr: true })`, `component$` App) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Native directory dialogs: `dialog.openDirectory` via `desktop.DialogService.OpenDirectory`, official dialog plugin, and Linux/Darwin/Windows hosts (GTK SELECT_FOLDER / NSOpenPanel directories / IFileOpenDialog FOS_PICKFOLDERS); scaffold + competitive bind the executor.
- Official `vitra.path` plugin: declares `path.open`; `desktop.PathService` opens absolute local paths via OS default handler (Linux `xdg-open`, Darwin `open`, Windows `explorer`); PathScope-gated; scaffold/generate/provenance/competitive bind.
- `vitra new --template angular`: Vite + Angular 19 + TypeScript starter (`@analogjs/vite-plugin-angular`, standalone `vitra-app` component) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Official `vitra.notification` plugin: declares `notifications.show`; `desktop.NotificationService` + Linux/Darwin/Windows hosts (D-Bus Notifications / NSUserNotification / tray balloon); scaffold, generate, provenance, and competitive demo bind title+body only.
- RPM/snap package metadata: `Spec.License` / `Homepage` / `Maintainer` / `Categories` flow into RPM `License:` / `URL:` / `Packager` / `Group:` and snap `license:` / `website:` / `contact:` (defaults match AppStream via `EffectiveLicense()` / `EffectiveMaintainer()` / `RPMGroup()`).
- Official `vitra.os` plugin: declares `os.info`; `desktop.OsService` returns GOOS/GOARCH/family/locale (stdlib default); scaffold, `vitra generate typescript`, packaging provenance inventory, and competitive demo register + bind alongside `fs`/`dialog`/`clipboard`/`browser`.
- Package metadata: `Spec.Homepage` / `Categories` / `Keywords` / `License` (+ `vitra package --homepage` / `--categories` / `--keywords` / `--license`) flow into FreeDesktop `.desktop` Categories/Keywords, AppStream metainfo (`<url>`, `<category>`, `<keyword>`, `<project_license>`), Debian `Homepage` + `Section:` + DEP-5 `copyright`, snap website/keywords, Windows ARP `URLInfoAbout` / `ARPURLINFOABOUT` / `HelpLink` / `ARPHELPLINK` / `URLUpdateInfo` / `ARPURLUPDATEINFO` / `LegalCopyright` / `ARPCOPYRIGHT`, and Darwin `NSHumanReadableCopyright`.
- `vitra new --template htmx`: Vite + HTMX + TypeScript starter (`htmx.org`, `hx-on:click` helpers) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- AppStream metainfo: Linux stages (dir/AppDir/deb/rpm/snap/flatpak) emit `usr/share/metainfo/<appid>.metainfo.xml` (or Flatpak `files/share/metainfo/`) for software centers; Flathub PR automation still out of scope.
- Native message dialogs: `dialog.message` (info/confirm) via `desktop.DialogService.Message`, official dialog plugin, and Linux/Darwin/Windows hosts (GTK MessageDialog / NSAlert / MessageBox); scaffold + competitive bind the executor.
- Official `vitra.browser` plugin: declares `browser.open`; scaffold, `vitra generate typescript`, packaging provenance inventory, and competitive demo register + bind via `desktop.BrowserService` alongside `fs`/`dialog`/`clipboard`.
- `vitra new --template alpine`: Vite + Alpine.js + TypeScript starter (`x-data` / Alpine.data, alpinejs) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Store publish execution: `packaging.ExecutePublish` + `vitra package --publish-execute` runs Executable plan steps only (`snapcraft upload`, `flatpak-builder`); never `snapcraft login` / Flathub `gh pr create`.
- Update signing CLI: `updater.GenerateKeyPair` / `BuildSignedManifest` / `LoadPrivateKeyRef` + `vitra update-keygen` / `update-sign` (env:/file:/secret: privkey refs; bare hex rejected).
- Update channel stage: `updater.StageChannel` + `vitra update-stage --out` writes `{out}/{app}/{channel}/manifest.json` + artifact for static CDN upload (digest-checked, signed manifests only).
- `vitra new --template lit`: Vite + Lit 3 + TypeScript starter (`vitra-app` LitElement, experimentalDecorators) embedding `frontend/dist` with a starter dist for immediate `vitra dev`.
- Flatpak portal policy: staged `metadata` `[Context]` / `[Session Bus Policy]` and `manifest.yml` finish-args talk to xdg-desktop-portal (FileChooser/Documents/OpenURI/Notifications) instead of `--filesystem=home`.
- Darwin notary credential bootstrap plan: `PlanNotaryCredentials` / `vitra notary-setup [--profile]` prints `notarytool store-credentials` argv (env placeholders only; never executed); Darwin `PlanSign` includes it as prep.
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
- Packaging `Spec.Maintainer` / `vitra package --maintainer`: sets Debian control Maintainer, snap `contact:`, and DEP-5 Upstream-Contact (default `Vitra Packaging <vitra@klarlabs.de>`) and WiX Manufacturer / NSIS Publisher when provided.
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

[Unreleased]: https://github.com/klarlabs-studio/vitra/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/klarlabs-studio/vitra/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/klarlabs-studio/vitra/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/klarlabs-studio/vitra/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/klarlabs-studio/vitra/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/klarlabs-studio/vitra/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/klarlabs-studio/vitra/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/klarlabs-studio/vitra/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/klarlabs-studio/vitra/releases/tag/v0.3.0
