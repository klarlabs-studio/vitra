# Desktop features

Files, dialogs, the clipboard, menus, the tray, notifications, and more come from the **official plugins** plus the capability-checked services in `desktop`.

- A plugin (`plugin/official`) declares commands, the permissions they need, and events. It contains no behaviour.
- A service (`desktop.ClipboardService`, …) checks the caller's grant again and calls the native host.
- You bind each command to its service. That is where you decide what the page can actually do.

See the [plugin reference](/reference/plugins) for every command and permission.

## Register the plugins

```go
for _, p := range official.All() { // or pick: official.Clipboard(), official.Dialog(), …
    if err := rt.RegisterPlugin(ctx, p); err != nil {
        return err
    }
}
```

Registering a plugin claims its permissions: no other plugin can declare them, so plugins cannot widen each other.

## Bind commands to services

```go
host := linux.New() // or darwin.New(), windows.New()

clips := &desktop.ClipboardService{
    Gateway: rt, Host: host,
    OnRead:  func(context.Context) (string, error) { return host.ClipboardGet() },
    OnWrite: func(_ context.Context, s string) error { return host.ClipboardSet(s) },
}
rt.BindExecutor("clipboard.read", domain.CallerExecutorFunc(
    func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
        return clips.Read(ctx, caller)
    }))

files := &desktop.FileService{Gateway: rt}
rt.BindExecutor("fs.read", domain.CallerExecutorFunc(
    func(ctx context.Context, caller domain.Caller, input any) (any, error) {
        path, _ := input.(string)
        b, err := files.Read(ctx, caller, path)
        return string(b), err
    }))
```

Each service re-checks the caller's grant (`clipboard.read`, `fs.read` with its path scope, …) before it acts, and returns `platform.ErrUnsupported` when the host can't do it.

`example/competitive` binds every official command. Copy from there.

## Then grant the permissions

```go
grant, _ := domain.NewCapabilityGrant("desktop", "clipboard and dialogs",
    []domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal},
    []domain.PermissionSpec{
        {Name: desktop.PermClipboardWrite},
        {Name: desktop.PermDialogOpen},
    })
```

Grant the least you can. Reading the clipboard, opening URLs, and scoped file access are each worth a second thought.

## Things the host does for the app

Menus, the tray, global shortcuts, and single-instance handoff are often set up by Go, not by the page. Call the service with a caller that represents the app and has its own grant:

```go
hostCaller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
menus.SetMenu(ctx, hostCaller, []desktop.MenuItem{
    {Menu: "File", ID: "app.quit", Label: "Quit", Shortcut: "Ctrl+Q"},
})
```

Command handlers never use this caller. They act as the window that called them.

## Platform support

Every host reports its features (`host.Features()`). Anything unavailable returns `platform.ErrUnsupported` with a reason instead of silently doing nothing. Notable cases:

- **Linux:** global shortcuts work on X11; on Wayland use in-window menu shortcuts.
- **macOS:** notifications need an app bundle with a bundle identifier. An unbundled binary (`go run`, `vitra dev`) gets `ErrUnsupported`; package the app with `vitra package --format app-dir`.
- **Windows:** requires the WebView2 runtime.
