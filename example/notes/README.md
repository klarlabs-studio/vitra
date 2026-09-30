# Vitra Notes

A Markdown notes app that shows the security model working. The page is an
ordinary web frontend. The Go side grants its window exactly this:

| Permission | Scope |
|---|---|
| `fs.read` | `<vault>/**`, except `<vault>/.private/**` |
| `fs.write` | `<vault>/**/*.md`, except `<vault>/.private/**` |
| `vault.info`, `audit.read` | where the vault is; the live audit log |

Everything else is refused before a handler runs, and shows up in the audit
panel with the reason.

```bash
make notes
# or
CGO_ENABLED=1 go run -tags vitra_native ./example/notes -vault ~/VitraNotes
```

An empty vault starts with a few notes (from `seed/`). Existing folders are
left as they are.

## Try to break it

The panel under the audit log runs real attacks through the real bridge:

| Attack | Stopped by |
|---|---|
| Read `/etc/hosts` | path scope: `path_out_of_scope` |
| Read `.private/credentials.md` | deny pattern: `path_denied` (any letter case) |
| `<vault>/../.ssh/id_ed25519` | path normalization: `..` is rejected |
| Save `payload.sh` into the vault | write scope allows `*.md` only |
| Get `Welcome.md` approved, read `/etc/hosts` | typed command: input path must equal the authorized path |
| Call `shell.exec` | only registered commands exist |
| Navigate the window to a remote site | navigation policy (`navigation.block`) |
| Embed a remote page in a frame | navigation policy applies to frames too |
| Forge a call from a sandboxed frame | sender token: dropped (`bridge.reject`) |

## Files

- `notes.go`: the grant and the four typed commands. File access goes
  through `desktop.FileService`, which resolves symlinks and authorizes the
  real target.
- `liveaudit.go`: an `audit.Sink` that pushes each event to the window.
- `frontend/`: plain HTML, CSS, and JS with no build step.
- `notes_test.go`: every attack above as a headless test.
- `e2e_linux_test.go`: the real UI in WebKitGTK (`make e2e-notes`, runs in CI).
- `tour_linux_test.go`: the paced tour behind the README recording.
  `scripts/record-notes-demo.sh` records it under Xvfb; the *Record demo*
  workflow does the same in CI and uploads the GIF.
