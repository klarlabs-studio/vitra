# Denial codes

A refused call rejects in the page with an `Error` whose `code` is one of these, and in Go returns a `*domain.ErrDenied` with the same `Code` and a `Reason`. Codes are stable; match on them, not on the reason text.

When several grants almost match, the most specific refusal wins: `path_denied`, then `path_out_of_scope`, `origin_mismatch`, and `window_mismatch`.

| Code | Meaning |
|---|---|
| `no_grant` | No grant covers this call |
| `window_mismatch` | A grant has the permission, but not for this window |
| `origin_mismatch` | A grant has the permission, but not for the window's current origin, or the claimed origin differs from the window's real one |
| `permission_absent` | The matching grant does not include this permission |
| `path_denied` | The path matches a deny pattern, or is malformed (`..`, NUL, UNC, device, relative) |
| `path_out_of_scope` | The path matches no allow pattern, a symlink leads outside the scope, or a typed input names a different path than the one authorized |
| `command_missing` | No such command is registered |
| `window_closed` | The calling window is closed |
| `unsupported_platform` | The navigation target is not supported on this platform |

Anything that is not a denial (your handler returned an error, a typed input failed to decode) has the code `error`. A malformed message from the page's own frame gets `bad_request`. A message dropped at the bridge gets no reply at all; it shows up in the audit log as `bridge.reject`.
