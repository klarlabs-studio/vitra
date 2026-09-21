# Phase 3 — Plugin SDK

Phase 3 stabilizes how privileged native surface enters a Vitra app: through
**declared plugins**, not ambient exports.

## Delivered

| Area | Location |
|------|----------|
| Manifest + SemVer compatibility | `plugin.Manifest`, `plugin.SemVer` |
| Contribution (commands/events) | `plugin.Contribution` |
| Registry + permission ownership | `plugin.Registry` |
| Lifecycle hooks | `plugin.Lifecycle` |
| Official `fs` / `dialog` / `clipboard` / `browser` / `os` / `notification` / `path` / `window` / `menu` / `tray` / `dragdrop` / `shortcut` / `app` contracts | `plugin/official/{fs,dialog,clipboard,browser,os,notification,path,window,menu,tray,dragdrop,shortcut,app}` |
| Runtime wiring | `Runtime.RegisterPlugin` / `BindExecutor` / `Plugins()` |
| TypeScript binding stub generator | `bindings.GenerateTypeScript` + `vitra generate typescript` |

Official plugins contribute **command definitions and permission ownership**.
Hosts bind executors (`BindExecutor`) for commands they implement — e.g. the
competitive demo binds `dialog.open`/`dialog.save`/`dialog.openDirectory`/`dialog.message` to native dialogs,
`clipboard.read`/`clipboard.write` to `desktop.ClipboardService`,
`browser.open` to `desktop.BrowserService`, `os.info` to `desktop.OsService`,
`notifications.show` to `desktop.NotificationService`,
`path.open` to `desktop.PathService`,
`window.create`/`window.close`/`window.chrome`/`window.getChrome` to `desktop.WindowService`
(via `app.App` / `DesktopHost` chrome APIs),
`menu.set` to `desktop.MenuService` (via `DesktopHost.SetMenuBar`; actions emit `menu.action`),
`tray.set` to `desktop.TrayService` (via `DesktopHost.SetTray`; actions emit `tray.action`),
`dragdrop.receive` to `desktop.DragDropService` (via `EnableDragDrop`; drops emit `dragdrop.drop`),
`shortcut.register` to `desktop.ShortcutService` (via `RegisterGlobalShortcut`; actions emit `shortcut.action`; Wayland unsupported),
`app.quit` to `desktop.AppService` (via `App.Quit`),
and `fs.read`/`fs.write` to
`desktop.FileService` with a PathScope grant.

## Security invariant 6

A plugin cannot silently expand another plugin’s permission scope. The
registry records a single owner per `PermissionName`; a second plugin that
declares the same permission is rejected with `ErrConflict`.

Commands contributed by a plugin must only reference permissions listed in
that plugin’s manifest.

## Binding generation

`bindings.GenerateTypeScript` emits a deterministic client stub tied to the
kernel version string so frontend SDKs cannot drift unnoticed (reliability
invariant 8).
