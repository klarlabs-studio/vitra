# How Vitra works

A Vitra app is a Go program that opens a native window showing a web page. The page can ask Go to do things. Vitra's job is to make sure it can only ask for what you allowed.

```text
Web page (untrusted)   window.vitra.invoke(command, input, resourcePath)
      │
      ▼
Native host            stamps the caller: (window ID, window origin)
      │                drops messages without the window's sender token
      ▼
Capability gateway     a grant must name this window, this origin,
      │                and the command's permission (and the path, if scoped)
      ▼
Your command           runs knowing who called it and what was authorized
```

## The page is untrusted

Treat everything in the WebView as potentially hostile: an XSS bug in your frontend, a compromised npm package, or remote content. Vitra does not try to make the page trustworthy. It limits what the page can reach.

## Commands are explicit

The page can call only commands you register with `vitra.Register` (or `Runtime.RegisterCommand`). Go methods are never exposed automatically. Each command declares the **permission** a caller needs.

## Grants give authority

A **grant** (`domain.CapabilityGrant`) says: these windows, showing these origins, may use these permissions. There is no "all windows" or "all origins" grant. A window without a matching grant can call nothing, not even a command that exists.

## Identity comes from the host

When a message arrives, the native host records which window it came from and that window's current origin. Anything the page puts in the message about who it is gets ignored. Each window also gets a random sender token that only its top frame knows, so an iframe cannot send calls as the page.

## Authority does not follow navigation

Grants name origins. If a window navigates away from your app's assets, it no longer matches its grants, and navigation to anything other than the app's own asset server is blocked anyway.

## Every refusal is deterministic and audited

A refused call gets a [denial code](/reference/denials) such as `no_grant` or `path_out_of_scope`, plus a reason. With an audit sink installed, every call is recorded, and so are refusals that happen before a command runs: forged bridge messages and blocked navigations.

## Where things live

| Package | What it is |
|---|---|
| `go.klarlabs.de/vitra` | The runtime: grants, commands, windows, events, updates |
| `app` | Runs an app: serves assets, opens windows, connects the host to the runtime |
| `domain` | Grants, path scopes, commands, denials (standard library only) |
| `desktop` | Capability-checked services: files, dialogs, clipboard, menus, … |
| `platform/linux`, `darwin`, `windows` | Native hosts (with `-tags vitra_native`) |
| `plugin/official` | Declarations of the built-in desktop commands and permissions |
| `audit`, `policy`, `updater`, `worker` | Audit sinks, enterprise policy, signed updates, supervised workers |

The [security model](/security) covers the threat model, the known limits, and the test behind each guarantee.
