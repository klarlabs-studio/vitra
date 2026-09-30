# Desktop features

Files, dialogs, the clipboard, menus, the tray, notifications, and more come from the **official plugins** plus the capability-checked services in `desktop`.

- A plugin (`plugin/official`) declares commands, the permissions they need, and events. It contains no behaviour.
- A service (`desktop.ClipboardService`, …) checks the caller's grant again and calls the native host.
- You bind each command to its service. That is where you decide what the page can actually do.

See the [plugin reference](/reference/plugins) for every command and permission.

## Turn them on

```go
a, err := app.New(app.Options{ /* ... */ })
if err != nil {
    return err
}
// Register every official plugin and bind its commands to the native host.
if err := a.UseOfficialPlugins(ctx); err != nil {
    return err
}
```

Or take only what you need: `a.UseOfficialPlugins(ctx, official.Clipboard(), official.Dialog())`. Call it after `app.New` and before `Run`.

This **grants nothing**. Every command still needs a grant for the calling window, and each service checks that grant again before it touches the host, including path scopes for `fs.*` and `path.open`.

It also forwards native activations: menu, tray, and shortcut activations become `menu.action`, `tray.action`, and `shortcut.action` events (`{"id": ...}`), and file drops become `dragdrop.drop` (`{"window", "paths"}`). [Subscribe](/guide/events) the windows that should receive them.

## Binding by hand

`UseOfficialPlugins` is a convenience over the pieces in `desktop`. To change how a command behaves, register the plugin yourself and bind its commands to a service:

```go
if err := rt.RegisterPlugin(ctx, official.Clipboard()); err != nil {
    return err
}
clips := &desktop.ClipboardService{
    Gateway: rt, Host: host,
    OnRead:  func(context.Context) (string, error) { return host.ClipboardGet() },
    OnWrite: func(_ context.Context, s string) error { return host.ClipboardSet(s) },
}
rt.BindExecutor("clipboard.read", domain.CallerExecutorFunc(
    func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
        return clips.Read(ctx, caller)
    }))
```

Registering a plugin claims its permissions, so no other plugin can declare them.

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
