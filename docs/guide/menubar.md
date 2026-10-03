# Menu bar apps

A menu bar app (a tray app on Linux and Windows) lives in the menu bar instead of the Dock or taskbar. It shows a live status next to its icon and drops down a panel when you click it, as [CodexBar](https://github.com/steipete/CodexBar) does. Vitra builds one from three pieces:

| Piece | API | What it does |
|---|---|---|
| Accessory presentation | `app.Options.Presentation` | No Dock icon or taskbar entry; closing the last window does not quit |
| Tray status | `App.SetTray`, `TraySpec` | Title and icon in the menu bar, plus a menu |
| Tray panel | `WindowKindPanel`, `TraySpec.Panel` | An HTML window that drops down from the icon |

The runnable version is [`example/menubar`](https://github.com/klarlabs-studio/vitra/tree/main/example/menubar): free disk space in the menu bar, with the details in a panel.

```bash
make menubar   # or: CGO_ENABLED=1 go run -tags vitra_native ./example/menubar
```

## 1. Run as an accessory app

```go
application, err := app.New(app.Options{
    AppID:        "com.example.usage",
    Assets:       assets,
    Host:         host,
    Presentation: app.PresentationAccessory,
    Window: app.WindowOptions{
        ID: "panel", Path: "/", Width: 340, Height: 280,
        Kind: app.WindowKindPanel,
    },
})
```

The primary window is the panel. `Run` opens it hidden, so the app shows nothing until the tray is set: call `SetTray` before `Run`, or `Run` refuses to start an app nobody could see or quit.

Package it with `vitra package --accessory` so macOS starts it without a Dock icon (`LSUIElement`).

## 2. Show a status in the tray

```go
err = application.SetTray(app.TraySpec{
    Title:    "87 GB free",          // next to the icon
    Tooltip:  "Macintosh HD: 413 GB of 500 GB used",
    Icon:     iconPNG,               // PNG bytes; draw or embed it
    Template: true,                  // macOS tints it to match the menu bar
    Panel:    "panel",               // a left click toggles the panel
    Items: []platform.MenuItem{     // the menu opens on a right click
        {ID: "refresh", Label: "Refresh"},
        {Separator: true},
        {ID: "quit", Label: "Quit"},
    },
})
application.OnAction(func(id string) {
    switch id {
    case "refresh":
        refresh()
    case "quit":
        application.Quit()
    }
})
```

Call `SetTray` again whenever the status changes, for example from a ticker. It needs no grant: it is your Go code, not the page.

A template icon is black with transparency at 2x size (36×36 pixels for the 18-point menu bar). Without `Panel` or `ClickActivates`, a click opens the menu instead.

## 3. Fill the panel

The panel is an ordinary page with an ordinary grant. Give it only what it needs:

```go
grant, _ := domain.NewCapabilityGrant("panel", "read the usage",
    []domain.WindowID{"panel"}, []domain.Origin{domain.OriginPackagedLocal},
    []domain.PermissionSpec{{Name: "usage.read"}})
```

Push updates to it with events (`App.Emit`), as the example's `usage.follow` command does. The panel also receives `tray.click` (`{"anchor": {x, y, width, height}}`) if it subscribes.

The panel hides itself when it loses focus. A second click on the icon hides it too, and Go can show or hide it at any time with `App.ShowTrayPanel` and `App.HideTrayPanel`, for example from a global shortcut. To let the page close its own panel, register a command that calls `HideTrayPanel`, as the example's `panel.close` does.

## Platform support

| | macOS | Windows | Linux (X11) | Linux (Wayland) |
|---|---|---|---|---|
| Title next to the icon | Menu bar text | Tooltip only | StatusNotifierItem label, depending on the panel | StatusNotifierItem label, depending on the panel |
| Icon | 18 pt, template | Small icon size | IconPixmap or GtkStatusIcon | IconPixmap |
| No Dock or taskbar entry | Yes | Yes | Yes | Yes |
| Icon position (`TrayAnchor`) | Yes | Yes | Yes | Only where the panel reports it |
| Panel under the icon | Yes | Yes, above it for a bottom taskbar | Yes | Centered: the compositor places windows |

Check `host.Features()` for `tray.title`, `tray.icon`, `tray.anchor`, `window.panel` and `app.presentation`; each `Detail` says how the host does it.

Linux shows tray icons through a StatusNotifierWatcher (KDE Plasma, GNOME with the AppIndicator extension, XFCE, and most other panels) and falls back to the XEmbed `GtkStatusIcon`. Stock GNOME without the extension shows no tray at all.
