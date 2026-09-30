# Official plugins

`plugin/official` declares the built-in desktop commands. `official.All()` returns every plugin; each also has its own constructor. A plugin declares commands and permissions only. `app.App.UseOfficialPlugins` binds them to the native host in one call; see [Desktop features](/guide/desktop).

| Plugin | Constructor | Permissions | Commands | Events |
|---|---|---|---|---|
| `vitra.fs` | `official.FS()` | `fs.read`, `fs.write` (path-scoped) | `fs.read`, `fs.write` | `fs.changed` |
| `vitra.dialog` | `official.Dialog()` | `dialog.open`, `dialog.save`, `dialog.openDirectory`, `dialog.message` | same as permissions | |
| `vitra.clipboard` | `official.Clipboard()` | `clipboard.read`, `clipboard.write` | same as permissions | |
| `vitra.browser` | `official.Browser()` | `browser.open` | `browser.open` (http, https, mailto) | |
| `vitra.os` | `official.OS()` | `os.info` | `os.info` | |
| `vitra.notification` | `official.Notification()` | `notifications.show` | `notifications.show` | |
| `vitra.path` | `official.Path()` | `path.open` (path-scoped) | `path.open` | |
| `vitra.window` | `official.Window()` | `window.create`, `window.close`, `window.chrome` | `window.create`, `window.close`; with `window.chrome`: `window.chrome`, `window.getChrome`, `window.focus`, `window.blur`, `window.hide`, `window.show`, `window.minimize`, `window.maximize`, `window.unmaximize`, `window.fullscreen`, `window.unfullscreen`, `window.setAlwaysOnTop`, `window.restore`, `window.setTitle`, `window.setSize`, `window.setIcon` | |
| `vitra.menu` | `official.Menu()` | `menu.set` | `menu.set`, `menu.clear` | `menu.action` |
| `vitra.tray` | `official.Tray()` | `tray.set` | `tray.set`, `tray.clear` | `tray.action` |
| `vitra.dragdrop` | `official.DragDrop()` | `dragdrop.receive` | `dragdrop.receive` | `dragdrop.drop` |
| `vitra.shortcut` | `official.Shortcut()` | `shortcut.register` | `shortcut.register`, `shortcut.unregister` | `shortcut.action` |
| `vitra.app` | `official.App()` | `app.quit` | `app.quit` | |
| `vitra.deeplink` | `official.DeepLink()` | `deeplink.handle` | | `deeplink.open` |

The permission constants live in `desktop` (`desktop.PermFSRead`, `desktop.PermClipboardWrite`, …).

## Writing a plugin

A plugin implements `plugin.Plugin`:

```go
type Plugin interface {
    Manifest() plugin.Manifest          // ID, name, version, permissions, minimum kernel
    Contribute() (plugin.Contribution, error) // commands and events
}
```

`rt.RegisterPlugin` checks the kernel version and refuses a plugin that declares a permission another plugin already owns. Bind executors for its commands with `rt.BindExecutor`, and grant its permissions like any other.
