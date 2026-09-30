# Security Policy

Vitra follows the [klarlabs-studio security policy](https://github.com/klarlabs-studio/.github/blob/main/SECURITY.md)
for reporting, response times, and disclosure. This file adds what is
specific to Vitra.

## Reporting a vulnerability

**Do not open a public issue.** Use
[private vulnerability reporting](https://github.com/klarlabs-studio/vitra/security/advisories/new)
(Security tab → **Report a vulnerability**), or email **security@klarlabs.de**.

Include the Vitra version or commit, the OS and host backend (WebKitGTK,
WKWebView, WebView2), and a minimal app or test that reproduces the issue.
A failing Go test against the kernel is the fastest path to a fix.

## Supported versions

Vitra is pre-1.0. Security fixes land on `main` and in the next release;
older minor versions are not patched.

## What counts as a vulnerability

Vitra's promise is that the web frontend is untrusted and gets only the
authority its grants name. Anything that breaks that is in scope, for example:

- **Gateway bypass:** invoking a command, or reaching a desktop service,
  without a grant that names the calling window, its origin, and the
  permission.
- **Identity spoofing:** a frontend making a call count as another window or
  origin, or keeping authority after navigating to another origin.
- **Path scope escape:** reading, writing, or opening a path outside a
  grant's allow patterns, or inside its deny patterns (traversal, case, symlinks,
  separators, UNC or device paths).
- **Navigation:** a page outside the app's asset server keeping bridge access.
- **Update verification:** installing an update that is unsigned, tampered,
  older than the installed version, or for another app or channel.
- **Native host bugs** that let page content run native code or read memory
  (for example command injection in openers or registry helpers).
- Memory or CPU exhaustion reachable from page content through the IPC bridge.

## What is not a vulnerability

- An app granting broad permissions (`/**` path scopes, `fs.write` plus
  `path.open` on one directory, every window). That is app configuration; the
  `vitra new` starter defaults to least privilege and documents the risks.
- An app enabling the WebView inspector (`app.Options.DevTools`) in a build it
  ships.
- Bugs in WebKitGTK, WebKit, or WebView2 themselves; report those upstream
  (we still want to hear about mitigations Vitra should apply).
- Anything that requires local code execution or control of the app's
  environment variables first.

## Security invariants

The properties above are encoded as tests. See `docs/intent.md` §9 and the
tests in `domain/`, `desktop/`, `app/`, `internal/ipc/`, and `updater/`,
including the fuzz targets (`go test -fuzz`) for path scopes, IPC decoding,
and version comparison.
