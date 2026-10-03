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

`dialog.open` takes `{"multiple": true}` to select several files and returns every selected path; a host that cannot select several files returns `platform.ErrUnsupported`. See [Picking files](/guide/desktop#picking-files).

The permission constants live in `desktop` (`desktop.PermFSRead`, `desktop.PermClipboardWrite`, …).

## Inputs and outputs

`UseOfficialPlugins` binds every command as a typed command (`vitra.Bind`) with the types below, from `plugin/official`. Input is decoded strictly: an unknown field, a wrong type, or a missing required value fails with a validation error (denial code `error`) before the host is touched. The generated client uses these types; commands with output `vitra.Void` send `null` and are typed `Promise<void>`.

| Command | Input | Output |
|---|---|---|
| `fs.read` | path `string` | `string` |
| `fs.write` | `WriteFileInput` `{path, data}` | `Void` |
| `dialog.open` | `DialogOptions` `{title?, defaultPath?, filters?, multiple?}` or `null` | `[]string` (`null` when cancelled) |
| `dialog.save`, `dialog.openDirectory` | `DialogOptions` or `null` | `string` |
| `dialog.message` | `MessageDialogInput` `{title?, message, kind?}` | `bool` |
| `clipboard.read` | none | `string` |
| `clipboard.write` | text `string` | `Void` |
| `browser.open` | URL `string` | `Void` |
| `os.info` | none | `desktop.OsInfo` |
| `notifications.show` | `NotificationInput` `{title?, body}` | `Void` |
| `path.open` | path `string` | `Void` |
| `window.create` | `WindowCreateInput` `{id, title?, path?, width?, height?}` | `WindowCreated` `{id}` |
| `window.chrome` | `WindowChromeInput` `{id, title?, width?, height?, maximized?, fullscreen?, alwaysOnTop?, minimized?, hidden?, iconPath?}` | `Void` |
| `window.getChrome` | `WindowRef` `{id}` | `platform.WindowChrome` |
| `window.setAlwaysOnTop` | `WindowAlwaysOnTopInput` `{id, alwaysOnTop}` | `Void` |
| `window.setTitle` | `WindowTitleInput` `{id, title}` | `Void` |
| `window.setSize` | `WindowSizeInput` `{id, width, height}` | `Void` |
| `window.setIcon` | `WindowIconInput` `{id, iconPath}` | `Void` |
| `window.close`, `window.focus`, `window.blur`, `window.hide`, `window.show`, `window.minimize`, `window.maximize`, `window.unmaximize`, `window.fullscreen`, `window.unfullscreen`, `window.restore` | `WindowRef` `{id}` | `Void` |
| `menu.set` | `MenuInput` `{items: [MenuItem]}`; `MenuItem` is `{id, label, menu?, shortcut?, disabled?, checked?}` or a separator `{separator: true, menu?}` | `Void` |
| `tray.set` | `TrayInput` `{tooltip?, title?, icon?, template?, items?}` | `Void` |
| `menu.clear`, `tray.clear`, `app.quit` | none | `Void` |
| `dragdrop.receive` | `DragDropInput` `{id?, enabled?}` (window `"main"`, enabled `true` by default) | `Void` |
| `shortcut.register` | `ShortcutInput` `{accelerator, action}` | `Void` |
| `shortcut.unregister` | `ShortcutRef` `{accelerator}` | `Void` |

The tray `icon` is the path of a PNG (at most 256 KiB) inside the app's assets, as the page would request it (`"/icons/tray.png"` or `"icons/tray.png"`). Paths with `..`, `.`, empty segments, backslashes or a drive letter are rejected, so the page can never point the tray at a file outside the app. `title` is text next to the icon where the platform has it (the `tray.title` feature); elsewhere it is shown in the tooltip. A menu separator takes no `id`, `label`, `shortcut`, `disabled` or `checked`; every other item needs `id` and `label`. Disabled items never fire, and a checked item stays checked until the menu is set again.

Window sizes must be whole numbers no larger than 2^53 (`window.setSize`: positive; `window.create` and `window.chrome`: `0` or absent leaves the size alone). Anything else is rejected, never rounded or ignored.

The client uses the object form of each input. `window.vitra.invoke` also accepts the older shorthands: a bare id string for `WindowRef` and `window.create`, a bare string for the `dialog.message` message, the `notifications.show` body, and the `shortcut.unregister` accelerator, a bare item array for `menu.set` and `tray.set`, a bare boolean for `dragdrop.receive`, `window` as an alias of `id` in `dragdrop.receive`, and `actionID` or `id` as aliases of `action` in `shortcut.register`.

## Writing a plugin

A plugin implements `plugin.Plugin`:

```go
type Plugin interface {
    Manifest() plugin.Manifest          // ID, name, version, permissions, minimum kernel
    Contribute() (plugin.Contribution, error) // commands and events
}
```

`rt.RegisterPlugin` checks the kernel version and refuses a plugin that declares a permission another plugin already owns. Bind typed handlers to its commands with `vitra.Bind` (or untyped executors with `rt.BindExecutor`), and grant its permissions like any other. See [Typing a plugin's commands](/guide/commands#typing-a-plugin-s-commands).
