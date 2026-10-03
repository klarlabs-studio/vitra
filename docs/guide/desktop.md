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

Every official command is a [typed command](/guide/commands). Its input is decoded strictly into a type in `plugin/official` (`official.DialogOptions`, `official.WindowSizeInput`, …): an unknown field or a wrong type fails with a validation error before the host is touched, and the [generated client](/guide/commands#the-typescript-client) types each call. See the [plugin reference](/reference/plugins#inputs-and-outputs) for every input and output.

## Picking files

`dialog.open` returns an array of paths, or `null` when the user cancels. It takes an optional object: `title`, `defaultPath`, `filters` (`[{"name": "Images", "extensions": ["png", "jpg"]}]`), and `multiple`:

```js
const paths = await window.vitra.invoke("dialog.open", {
  title: "Add attachments",
  multiple: true,
  filters: [{ name: "Documents", extensions: ["pdf", "md"] }],
});
```

Without `multiple` (or with `false`) the dialog selects one file, as before; a `multiple` that is not a boolean is rejected. With `multiple: true` the Linux, macOS, and Windows hosts let the user select several files and return all of them. A host that cannot do that returns `platform.ErrUnsupported`; it never quietly returns a single file. Custom hosts opt in by implementing `platform.MultiFileOpener`.

Picking a file grants nothing. Reading it with `fs.read` still needs an `fs.read` grant whose path scope covers it.

## Binding by hand

`UseOfficialPlugins` is a convenience over the pieces in `desktop`. To change how a command behaves, register the plugin yourself and bind its commands to a service with `vitra.Bind`, which gives a plugin's command a typed handler:

```go
if err := rt.RegisterPlugin(ctx, official.Clipboard()); err != nil {
    return err
}
clips := &desktop.ClipboardService{
    Gateway: rt, Host: host,
    OnRead:  func(context.Context) (string, error) { return host.ClipboardGet() },
    OnWrite: func(_ context.Context, s string) error { return host.ClipboardSet(s) },
}
err := vitra.Bind(rt, "clipboard.read",
    func(ctx context.Context, inv domain.Invocation, _ struct{}) (string, error) {
        return clips.Read(ctx, inv.Caller)
    })
```

Pass `inv.Caller` to the service, never an identity from the input. For input, reuse the types in `plugin/official` so your binding decodes and validates exactly as the built-in one does. Use `vitra.Void` as the output of a command that returns nothing.

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

- **Linux:** global shortcuts work on X11 (`XGrabKey`). On Wayland they go through the `org.freedesktop.portal.GlobalShortcuts` portal (GNOME 48+, KDE Plasma 6, other desktops whose `xdg-desktop-portal` backend implements it). `Features()` reports the shortcut feature available only while that portal is running; without it `shortcut.register` returns `ErrUnsupported`, so fall back to in-window menu shortcuts (`MenuItem.Shortcut`). Through the portal:
  - The accelerator is the *preferred* key. The desktop may show a consent dialog, and the user can approve, change the key, or refuse. A refusal or cancel is returned as an error from `shortcut.register`.
  - Each action id is one portal shortcut, so registering a second accelerator for the same action replaces the first.
  - The portal cannot unbind a single shortcut. `shortcut.unregister` rebinds the remaining shortcuts on a new portal session, which the desktop may confirm with the user again.
  - Desktops attribute shortcuts to an app id. An unsandboxed app with a reverse-DNS program name (`host.SetProgramName("com.example.Notes")`, matching its `.desktop` file) registers it with the portal, so the user's choices are remembered between runs. Flatpak apps get their id from the sandbox.
- **macOS:** notifications need an app bundle with a bundle identifier. An unbundled binary (`go run`, `vitra dev`) gets `ErrUnsupported`; package the app with `vitra package --format app-dir`.
- **Windows:** requires the WebView2 runtime.
- **Linux tray:** when a `org.kde.StatusNotifierWatcher` is on the session bus, the tray is a StatusNotifierItem with a `com.canonical.dbusmenu` menu, spoken directly over D-Bus with no extra library. KDE Plasma, GNOME with the AppIndicator extension (on by default in Ubuntu, Fedora needs it installed), XFCE, Cinnamon, MATE, LXQt, Budgie, and panels such as waybar show it, on X11 and Wayland. Without a watcher, Vitra falls back to the legacy XEmbed `GtkStatusIcon`, which only X11 panels with a system tray show; stock GNOME without the extension shows no tray at all. `Features()` says which protocol is in use. Either way, menu clicks emit `tray.action` with the item's ID and a left click on the icon emits `tray.action` with `tray.activate`. Menus are flat lists, labels are shown literally, and the icon is the generic `application-x-executable`.

A custom host implements only the capabilities it supports; a command whose capability is missing fails with `ErrUnsupported`. See [Host interfaces](/reference/hosts).
