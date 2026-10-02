# Getting started

## Requirements

- Go 1.26 or later, with cgo enabled for the native hosts.
- A WebView:
  - **Linux:** WebKitGTK 4.1 (`sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev pkg-config`).
  - **macOS:** 11 or later. WKWebView ships with the OS; the Xcode command line tools provide cgo.
  - **Windows:** 10 or 11 with the [WebView2 Evergreen Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (preinstalled on Windows 11).

`vitra doctor` checks these for you.

## Install the CLI

```bash
go install go.klarlabs.de/vitra/cmd/vitra@latest
vitra doctor
```

## Create an app

```bash
vitra new hello              # plain HTML frontend, no build step
cd hello
vitra dev                    # runs the app, restarts on changes, inspector enabled
```

`vitra new --template vite|react|svelte|vue` creates a Vite frontend instead. The starter includes a prebuilt `frontend/dist`, so `vitra dev` works before you run `npm install`.

You get a window with a name field and a **Greet** button. Clicking it calls the Go command `greet`.

## What was generated

```text
hello/
├── main.go                 # the app: one command, one grant, the runner
├── go.mod
├── frontend/
│   ├── index.html          # the page (or a Vite project with dist/)
│   └── vitra-client.ts     # typed client generated from main.go
└── README.md
```

`main.go` is about 120 lines. The parts that matter:

```go
// An explicit, typed command.
vitra.Register(rt, vitra.Command[GreetRequest, Greeting]{
    Name:       "greet",
    Permission: "greet",
    Handler: func(ctx context.Context, inv domain.Invocation, req GreetRequest) (Greeting, error) {
        return Greeting{Message: "Hello, " + req.Name}, nil
    },
})

// The grant: the main window, showing the app's own assets, may call greet.
grant, err := domain.NewCapabilityGrant(
    "main-window", "what the main window may do",
    []domain.WindowID{"main"},
    []domain.Origin{domain.OriginPackagedLocal},
    []domain.PermissionSpec{{Name: "greet"}},
)
```

Remove the grant and the button fails with `no_grant`. Nothing in the frontend can change that.

## Start with desktop plugins

To begin with files, dialogs, or the clipboard, name the plugins with `--with`:

```bash
vitra new hello --with fs,dialog,clipboard
```

`main.go` then registers exactly those [official plugins](/guide/desktop) with `a.UseOfficialPlugins(ctx, official.FS(), official.Dialog(), official.Clipboard())` and grants each one as narrowly as a starter can. The page gets one button per plugin that makes a real call.

| `--with` | Granted | Left out on purpose |
|---|---|---|
| `fs` | `fs.read` and `fs.write`, scoped to one data folder under your home (`os.UserConfigDir()/<app id>`, symlinks resolved), plus a `dataDir` command that tells the page where it is | Any other folder. Narrowing writes to `dataDir + "/**/*.md"` is a commented option |
| `dialog` | `dialog.open`, `dialog.save` | `dialog.openDirectory`, `dialog.message` |
| `clipboard` | `clipboard.write` | `clipboard.read`, which would show the page whatever you copied elsewhere |
| `notification` | `notifications.show` | |
| `os` | `os.info` | |

A picked file is not a grant: reading or writing it still needs a scoped `fs` permission on its folder. `path.open` is never granted, and should never be granted on a folder the page can write. Plugins that need app-specific scopes or ids (`path`, `browser`, `window`, `menu`, `tray`, `shortcut`, `dragdrop`, `app`, `deeplink`) are not offered by `--with`; add them by hand as described in [Desktop features](/guide/desktop). An unknown name is an error that lists the valid ones.

## Build and run without the CLI

```bash
CGO_ENABLED=1 go run -tags vitra_native .
CGO_ENABLED=1 go build -tags vitra_native -o hello .
```

Without `-tags vitra_native` the platform packages compile to stubs that report every feature as unsupported. That keeps `go build ./...` and `go test ./...` working on machines without a WebView.

## Next steps

- [How Vitra works](/guide/concepts): the model behind the two snippets above.
- [Commands](/guide/commands) and [grants](/guide/grants): add your own.
- [Desktop features](/guide/desktop): files, dialogs, clipboard, menus, the tray, and more.
- [Examples](/guide/examples): the notes app and the full feature tour.
