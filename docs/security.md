# Vitra security model

This page explains what Vitra protects, how it decides what a page may do,
where the known limits are, and which tests enforce each property. To report
a problem, see [SECURITY.md](https://github.com/klarlabs-studio/vitra/blob/main/SECURITY.md).

## Threat model

Vitra assumes the **web frontend is hostile**. Page content can be attacker
controlled through:

- an XSS bug in the app's own frontend,
- a compromised npm dependency bundled into the frontend,
- remote content the app navigates to or embeds.

The Go process, the app author's grants, and the OS user are trusted. The
goal: whatever runs in the page gets **only the authority the app's grants
name**, for the window it runs in, at the origin it runs at. Nothing more.

```text
Web frontend (untrusted)  ── window.vitra.invoke(command, input, resourcePath)
        │  one JSON message, ≤ 1 MiB
        ▼
Native host (WebKitGTK / WKWebView / WebView2)
        │  stamps caller = (window ID, window origin); payload claims ignored
        ▼
Capability gateway  ── grant must name window + origin + permission
        │               path-scoped permissions also check the resource path
        ▼
Command executor (explicitly registered)  ── acts as the authorized caller
        ▼
Desktop services (fs, dialogs, clipboard, …)  ── re-authorize the exact
                                                  resource, resolving symlinks
```

## How a call is authorized

1. **Identity comes from the host, not the page.** The native bridge stamps
   each message with the window it arrived in and that window's current
   origin. `window`/`origin` fields in the payload are discarded
   (`internal/ipc`).
2. **Commands are explicit.** Only commands registered with
   `Runtime.RegisterCommand` or `vitra.Register` are callable; Go methods are
   never exported automatically.
3. **Grants are narrow.** A `domain.CapabilityGrant` names windows, origins,
   and permissions, optionally with a path scope. There is no "all windows" or
   "all origins" grant. A new window has no authority until a grant names it.
4. **Authority does not follow navigation.** When a window navigates to
   another origin, grants for the old origin no longer match.
5. **Executors act as the caller.** The authorized `domain.Invocation`
   (caller, command, checked resource path, grant) is on the executor's
   context; `domain.CallerExecutorFunc` and typed `vitra.Command` handlers
   receive it. A typed input that names a resource (`ResourcePath()`) must
   equal the path the gateway checked.
6. **Denials are deterministic and audited.** Every refusal has a
   `domain.DenialCode` (`no_grant`, `window_mismatch`, `origin_mismatch`,
   `path_denied`, `path_out_of_scope`, …) and a reason, and is recorded by
   the audit sink. So are refusals before a command runs: dropped bridge
   messages (`bridge.reject`) and blocked navigations (`navigation.block`,
   logged without credentials, query, or fragment).
7. **Enterprise policy can only tighten.** A `policy.Engine` overlay can turn
   an allow into a deny, never the reverse.

## Path scopes

Path-scoped permissions (`fs.read`, `fs.write`, `path.open`) carry allow and
deny patterns.

**Normalization.** Backslashes are separators everywhere. A path must be
absolute (`/a/b` or `C:/a/b`). These are rejected:

- `..` segments,
- NUL bytes,
- relative and drive-relative paths,
- UNC paths (`\\host\share`) and device paths (`\\?\`).

**Patterns.** `**` matches any number of whole segments. Every other segment
is a `path.Match` glob, so `*.pem` matches one file name and `*` matches one
segment. Patterns must be absolute and valid, or `NewCapabilityGrant`
rejects the grant.

**Deny wins, case-insensitively.** Any matching deny refuses the path. Deny
patterns ignore letter case, so `/project/.SECRETS` cannot slip past a
`.secrets` deny on APFS or NTFS. Allow patterns are case-sensitive, so an
ambiguous path fails closed both ways.

**Aliases.** A directory can have several absolute spellings (macOS `/var`
is `/private/var`). Deny patterns cover both: when a grant is registered,
each deny pattern's folder is resolved and its real spelling added as
another deny (shown by `inspect`), and file operations also check the
resolved real path of every file against the deny patterns. Allow patterns
are never widened this way.

**Unicode.** macOS (APFS) treats composed and decomposed spellings of a
name (`é` as one code point or as `e` plus an accent) as the same file;
Linux and Windows treat them as different files. Vitra compares names in the
spelling stored on disk, which macOS reports from an open handle: deny
folders are resolved to it at registration, and so is the directory of
every file opened. A deny written in one spelling therefore also refuses
the other, without bundling Unicode tables into the kernel.

**Symlinks.** Scopes compare strings. The desktop file and path services
resolve the real target before acting:

1. The target must stay under the real root of the allow pattern that
   matched.
2. Its location, mapped back under that root, is authorized again, so deny
   rules apply to what a link points at.
3. Dangling links are refused for writes.

**No check-then-use race for files.** `desktop.FileService` does not reopen
the checked path. It opens the file through an `os.Root` at the scope's real
root, so the kernel refuses anything that leaves the scope however
directories are swapped in the meantime. It then asks the OS where the open
file actually is (`/proc/self/fd` on Linux, `F_GETPATH` on macOS,
`GetFinalPathNameByHandle` on Windows) and authorizes that location again,
deny patterns included. All I/O goes through that handle. A new file is
created exclusively inside the handle of the directory that was authorized,
so nothing is ever created elsewhere. Where the OS cannot report a file's
path, the opened file must be the one identified at authorization.

**Known limits:**

- **Aliases created after registration.** Deny patterns cover their folder's
  real spelling as resolved when the grant is registered. A symlink created
  later that renames a denied folder is still caught when files are opened
  (the real path is checked), but plain `Authorize` calls only see the
  spellings known at registration.
- **Unicode in names that do not exist yet.** Deny patterns compare folder
  names in the spelling stored on disk (see "Unicode" above), so the
  folders must exist when the grant is registered or when a file in them is
  opened. Non-ASCII glob parts (`/x/**/Café.txt`) are compared as written.
- **The opener's own open.** `path.open` is verified like a read: the target
  is opened through the scope, its real location is authorized, and the
  opener receives that symlink-free location only after checking it still
  leads to the same file. The OS's default application then opens the path
  itself (no platform opener accepts a file handle), so a process that
  swaps a directory in the instant after this last check could still
  redirect it. That needs local code execution racing a millisecond
  window; page content alone cannot do it.

**Never grant `fs.write` and `path.open` on the same directory.** Opening a
file with its default handler runs executables and scripts, so together they
let a page write a program and launch it. The `vitra new` starter grants
neither by default.

## Navigation and origins

- The asset server is on `127.0.0.1` with a random port. Navigation is
  allowed to exactly that scheme and `host:port` (no userinfo) or to
  `about:blank`. Anything else loses the bridge. Earlier versions used a
  string-prefix check that `http://127.0.0.1:PORT@evil.example/` passed.
- **Only the top frame can call.** Each window gets a random 256-bit sender
  token. It is embedded in the preload's closure, which runs only in the top
  frame, and never placed on `window`. Every message must carry it (checked
  in constant time). Messages without it are dropped without a reply. This
  covers subframes that can reach the native message handler, including
  cross-origin iframes: WebView2 injects the preload into every frame, but
  the script returns in subframes before defining the token. macOS also
  drops messages whose `frameInfo` is not the main frame.
- **Every message is checked against the document that sent it.** The
  native hosts report the sender with each message (WebView2's message
  source, WKWebView's frame request URL, WebKitGTK's page URI). The app
  accepts a message only if that document was served by its own asset
  server, and derives the call's origin from it. Anything else (a remote
  page, `about:blank`, another local port) is dropped without a reply and
  audited as `bridge.reject`, so authority does not depend on the navigation
  policy alone. Custom hosts that do not report the sender fall back to the
  origin recorded for the window.

This is the class of bug behind Tauri's recent origin CVEs:

- [GHSA-57fm-592m-34r7](https://github.com/tauri-apps/tauri/security/advisories/GHSA-57fm-592m-34r7):
  iframes bypassed origin checks.
- [GHSA-7gmj-67g7-phm9](https://github.com/tauri-apps/tauri/security/advisories/GHSA-7gmj-67g7-phm9):
  local-URL confusion let remote pages reach local-only IPC.

Vitra's design keys authority on host-recorded identity, a top-frame sender
token, and a per-message check of the sending document, parsed exactly
rather than compared as URL strings.

## Other defenses

- **IPC limits.** Inbound messages over 1 MiB are rejected before JSON
  parsing. Nesting depth is bounded by `encoding/json`. Invokes run off the
  UI thread, so a slow or hostile call cannot freeze the window.
- **No inspector by default.** The WebView inspector is off unless
  `app.Options.DevTools` is set or `VITRA_DEVTOOLS=1` (which `vitra dev`
  sets).
- **Openers never use a shell.** `browser.open` accepts http, https, and
  mailto, and launches them without a command interpreter (Windows uses
  `rundll32 url.dll,FileProtocolHandler`, not `cmd /c start`). Generated
  `.reg` values escape quotes and drop newlines.
- **Updates.** A manifest must carry a valid ed25519 signature, match the
  artifact's SHA-256, be for the installed app and channel, and have a
  strictly newer SemVer version, and not be past its signed `expires_at`
  (90 days by default, `vitra update-sign --expires-in`). Otherwise it is
  refused, so old signed releases cannot be replayed and a mirror cannot keep
  serving a stale one forever.
  Installing never writes outside the destination: archive entries that
  are absolute, contain `..`, or are symlinks leaving the archive are
  rejected, and unpacked size and entry count are bounded. Channels require HTTPS (loopback excepted).
  Signing keys never live in project configuration.

## Invariants and the tests that enforce them

`docs/intent.md` lists the invariants. These tests fail if one breaks:

| Property | Tests |
|---|---|
| New windows have no authority (1) | `domain`: `TestGateway_NewWindowHasNoPrivileges` |
| Navigation drops authority (2) | `domain`: `TestGateway_NavigationDropsAuthority`; `app`: `TestApp_RunInvokeAndNavPolicy` (exact-origin nav, userinfo and adjacent-port bypasses) |
| Frontend input not trusted (3) | `vitra`: `TestRegister_RejectsMalformedInput`, `TestRegister_BindsInputResourcePathToAuthorizedPath`; `vitra`: `TestBind_TypesAPluginCommand`; `plugin/official`: `TestInputs_RejectUnknownFieldsWrongTypesAndMissingValues`, `TestWindowSizes_RejectInexactOrOutOfRangeNumbers`, `FuzzMenuInput`, `FuzzTrayInput`, `FuzzDialogOptions`, `FuzzShortcutInput`, `FuzzShortcutRef`, `FuzzDragDropInput`, `FuzzWindowCreateInput`, `FuzzWindowRef`, `FuzzWindowAlwaysOnTopInput`, `FuzzWindowTitleInput`, `FuzzWindowSizeInput`, `FuzzWindowIconInput`, `FuzzWindowChromeInput`, `FuzzMessageDialogInput`, `FuzzNotificationInput`; `app`: `TestUseOfficialPlugins_DecodesInputStrictly` |
| Host-stamped identity (4) | `internal/ipc`: `TestBridge_DiscardsSpoofedCallerIdentity`, `FuzzBridge_DecodeInvoke`; `domain`: `TestInvocationService_RejectsSpoofedOrigin`, `TestCallerExecutorFunc_ReceivesInvokingWindow`, `TestCallerExecutorFunc_FailsClosedWithoutInvocation` |
| Plugins cannot widen each other (6) | `plugin`: `TestRegistry_RejectsPermissionCollision` |
| No shell by default (7) | `platform/windows`: `TestOpenURL_NeverInvokesShell`, `TestRegEscape` |
| Verified updates (9) | `updater`: `TestPlanInstall_RequiresValidSignatureAndDigest`, `TestPlanInstall_RequiresNewerVersionSameAppAndChannel`, `TestChannelSource_RequiresHTTPSExceptLoopback`, `TestPlanInstall_RejectsExpiredOrUndatedManifests`, `TestManifest_ExpiryIsSigned`, `TestApplyInstall_RejectsUnsafeArchives`, `TestApplyInstall_EnforcesExtractionLimit`, `FuzzCompareVersions`; `policy`: `TestEngine_AuthorizeUpdate` |
| Signing secrets outside config (10) | `internal/packaging`: `TestSpec_RejectsInlineSigningSecretPattern` |
| Dev privileges stay out of release (11) | `policy`: `TestEngine_ProductionForcesSignatureAndBlocksDevPerms`; `app`: `TestRun_DevToolsOffByDefault` |
| Remote content has no authority (12) | `domain`: `TestGateway_RemoteContentDeniedByDefault`, `TestNavigationPolicy_UntrustedDeniedByDefault` |
| Deterministic denials (13) | `domain`: `TestGateway_DenialDeterministicAndInspectable`; `app`: `TestApp_AuditsRejectedMessagesAndBlockedNavigation` |
| Unsupported behavior is explicit (14) | `internal/platform/null`: `TestNullHost_ExplicitUnsupportedDialog` |
| Path scopes | `domain`: `TestPathScope_RejectsBypasses`, `TestPathScope_WindowsDrivePaths`, `TestNewCapabilityGrant_RejectsMalformedPathPatterns`, `FuzzPathScope_Matches`; `desktop`: `TestFileService_ReadFollowsSymlinksOnlyWithinScope`, `TestFileService_WriteCannotEscapeThroughSymlinks`, `TestFileService_ReadCannotBeRacedOutOfScope`, `TestFileService_WriteCannotBeRacedOutOfScope`, `TestFileService_RaceWithoutFdPathLookup`, `TestPathService_OpenCannotBeRacedOutOfScope`, `TestPathService_OpenPassesTheVerifiedPath`; `domain`: `TestPathScope_WithDenyAliases`; `vitra`: `TestRegisterGrant_DenyCoversRealSpelling`; `desktop`: `TestFileService_DenyOnRealPathAppliesThroughAlias`, `TestFileService_DenyIgnoresUnicodeSpelling` |
| Least-privilege starter | `cmd/vitra`: `TestScaffold_GrantsLeastPrivilegeByDefault` |
| IPC limits | `internal/ipc`: `TestBridge_RejectsOversizedMessages` |
| Every message's sender is checked | `app`: `TestApp_ChecksSenderOfEveryMessage` |
| Only the top frame can call | `internal/ipc`: `TestBridge_RequiresSenderToken`; `internal/bridge`: `TestPreload_KeepsSenderTokenInTopFrameClosure`; `app`: `TestApp_RunInvokeAndNavPolicy` (forged tokens get no reply) |
| Host calls from commands don't deadlock | Native E2E (`make e2e`): clipboard round-trip from an invoke |

Run the fuzz targets locally with, for example,
`go test ./domain -run '^$' -fuzz FuzzPathScope_Matches -fuzztime 60s`.
The official input targets run one at a time the same way, for example
`go test ./plugin/official -run '^$' -fuzz '^FuzzWindowSizeInput$' -fuzztime 30s`.
