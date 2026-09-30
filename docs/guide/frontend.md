# The frontend

Any static site works: plain HTML, or anything that builds to a folder. Vitra embeds the files into your binary and serves them from `127.0.0.1` on a random port.

```go
//go:embed all:frontend/dist
var frontendRoot embed.FS

assets, _ := fs.Sub(frontendRoot, "frontend/dist")
a, _ := app.New(app.Options{Assets: assets, /* ... */})
```

## The bridge

Vitra injects `window.vitra` before your scripts run:

```ts
window.vitra.invoke(command: string, input?: unknown, resourcePath?: string): Promise<unknown>
window.vitra.on(event: string, handler: (payload: unknown) => void): () => void  // returns unsubscribe
```

A refused call rejects with an `Error` whose `code` is the [denial code](/reference/denials) (or `error` for a failure in your handler):

```ts
try {
  await window.vitra.invoke("notes.read", { path }, path);
} catch (err) {
  if (err.code === "path_denied") showLocked();
}
```

Use the [generated client](/guide/commands#the-typescript-client) for typed calls.

## Starter templates

`vitra new --template` gives you a working project:

| Template | Frontend |
|---|---|
| `vanilla` (default) | One HTML file, no build step |
| `vite` | Vite + TypeScript |
| `react` | Vite + React + TypeScript |
| `svelte` | Vite + Svelte 5 + TypeScript |
| `vue` | Vite + Vue 3 + TypeScript |

For other frameworks, start from `vite` and point your build at `frontend/dist`.

## Navigation

The window may show only your asset server and `about:blank`. Everything else is refused and audited as `navigation.block`: top-level navigation, remote iframes, and look-alikes such as `http://127.0.0.1:PORT@evil.example/` or another local port. To open a web page, use the `browser.open` command, which hands the URL to the system browser.

## Frames

Only the top frame of your page can call Go. Each window gets a random sender token that lives in the injected script's closure and is sent with every call. Frames never see it, and calls without it are dropped and audited as `bridge.reject`. A same-origin iframe can still reach `window.top.vitra`, since it is your own code. Put third-party content in a `sandbox` iframe.

## Content Security Policy

Vitra enforces its limits at the bridge regardless of what the page does, but a CSP still makes XSS much harder. A good start for an app without remote resources:

```html
<meta http-equiv="Content-Security-Policy"
      content="default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:">
```

## The inspector

The WebView inspector is off by default. `vitra dev` turns it on, as do `app.Options{DevTools: true}` and the `VITRA_DEVTOOLS=1` environment variable. Don't ship a build with it on: anyone at the keyboard could run script with the page's authority.
