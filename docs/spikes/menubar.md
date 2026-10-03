# Menu Bar Apps — Plan

Status: **planned**. Goal: build a CodexBar-style app with Vitra, meaning one
that runs only from the menu bar or tray, shows a live title or icon there, and
opens a rich HTML panel under the icon when you click it.

## Gap analysis (v0.9.0)

| Need | Today | Gap |
|------|-------|-----|
| Live status text / icon | `tray.set{tooltip, items}`; darwin title hard-coded `"vitra"` | No title, no icon, no template image |
| Rich menu | Flat `MenuItem{ID, Label, Menu, Shortcut}` | No separators, disabled, checked items |
| Go-driven updates | Tray only via the `tray.set` frontend command | No `App` Go API; polling lives in Go |
| No Dock / taskbar entry | darwin `ActivationPolicyRegular` always | No accessory presentation, no `LSUIElement` |
| Click → panel | Click always opens the native menu | No primary-click event, no anchor rect |
| Popover window | `WindowOptions{ID, Title, Width, Height, Path}` | No frameless, positioned, hide-on-blur panel |

## Design principles

The kernel invariants stay as they are (`docs/intent.md` §9):

- **The panel is an ordinary window.** It gets a `WindowID` stamped by the host,
  has no grants by default, and keeps the same navigation and origin rules.
  Being anchored to the tray icon gives it no extra authority.
- **Tray icons come from the app's `Assets` FS** and are resolved in Go. The
  frontend never passes a filesystem path, so `tray.set` cannot read files
  outside the app bundle.
- **Go-side tray APIs are trusted app code**, just like `App.OpenWindow`.
  Frontend `tray.set` stays behind the `tray.set` permission.
- **Unsupported is explicit.** Each new behaviour gets a `platform.Feature`
  with a `Detail`, and a host that can't do it returns `ErrUnsupported`
  instead of approximating.

## Milestones

Each milestone is one PR and adds its own tests, docs and CHANGELOG entry.

### M1 — Tray status: title, icon, richer items (`feat(tray)!`) — delivered

- `platform.TraySpec{Tooltip, Title, Icon []byte, Template bool, Items []MenuItem}`;
  `Tray.SetTray(TraySpec)` replaces `SetTray(tooltip, items)`. This is a
  breaking change, which is allowed pre-1.0.
- `platform.MenuItem` gains `Separator`, `Disabled`, `Checked`. Apply these to
  `menu.set` as well so both menus share one model.
- `official.TrayInput` gains `title`, `icon` (an asset path inside `Assets`,
  validated with the same `..` rejection as path scopes) and `template`.
- `(*App).SetTray(ctx, TraySpec)` and `(*App).ClearTray()` let Go code
  (pollers, workers) update the status without a page round-trip.
- Hosts:
  - darwin: `button.title`, and `button.image` from PNG bytes with
    `image.template` set.
  - Linux SNI: `Title` and `IconPixmap` (decode the PNG in Go, then emit
    `NewTitle`/`NewIcon`). Set `XAyatanaLabel` too, since that's where GNOME
    and Ubuntu show text. The GtkStatusIcon fallback shows the icon only.
  - Windows: `NIM_MODIFY` with `HICON` built from the PNG. Windows trays can't
    show text, so `Title` goes into the tooltip, and the Feature `Detail` says
    so.
- Tests: input fuzz for the new fields, icon path traversal is rejected, a
  fake host records the `TraySpec`, and native unit tests cover PNG → pixmap
  conversion.

### M2 — Accessory presentation (`feat(app)`) — delivered

- `app.Options.Presentation`: `PresentationRegular` (default) or
  `PresentationAccessory`.
- New `platform.Presentation` interface with Feature `app.presentation`:
  - darwin: `NSApplicationActivationPolicyAccessory`
  - Windows: app windows owned by a hidden window (no taskbar button, normal caption)
  - Linux: `gtk_window_set_skip_taskbar_hint` and `skip_pager`
- Accessory mode with no main window: `Options.Window` becomes optional when
  `Presentation == Accessory`. Tray actions keep the app alive, and `Quit`
  comes from a tray item.
- Packaging: `Spec.Accessory` (`vitra package --accessory`) → `LSUIElement` in the darwin Info.plist.
- Tests: option validation (an accessory app with no window and no tray is an
  error at `Run`), and that the packaging plist contains the key.

### M3 — Tray activation and anchor (`feat(tray)`) — delivered

- Split clicks with `TraySpec.ClickActivates`: a primary click emits
  `tray.click` with `{anchor}` and a secondary click opens the menu. Without
  it, clicks behave as before (menu on macOS, `tray.activate` elsewhere).
- `platform.TrayAnchor() (Rect, error)` returns the icon's screen rect:
  - darwin: `button.window.frame`
  - Windows: `Shell_NotifyIconGetRect`
  - Linux SNI: the `x, y` passed to `Activate`
  - GtkStatusIcon: `gtk_status_icon_get_geometry`
  - Wayland usually gives no rect, so it reports `ErrUnsupported` with a reason.
- Tests: action routing (click vs menu item), and anchor plumbing through the
  null host.

### M4 — Tray panel window (`feat(window)`) — delivered

- `WindowOptions.Kind`: `WindowKindNormal` | `WindowKindPanel`. A panel is
  frameless, non-resizable, always on top, has no taskbar entry, and hides
  when it loses focus.
- `TraySpec.Panel domain.WindowID`: a primary click toggles that panel, and the
  host positions it under the click's anchor (or `TrayAnchor()`), clamped to
  the visible screen. Go gets `App.ShowTrayPanel` / `App.HideTrayPanel` (for
  global shortcuts); the page closes its panel with `window.hide`.
- A menu bar app makes its primary window the panel (`Window.Kind`); `Run`
  opens it hidden and requires a tray.
- Hosts:
  - darwin: `NSPanel` with `NSWindowStyleMaskBorderless |
    NSWindowStyleMaskNonactivatingPanel`, `becomesKeyOnlyIfNeeded = NO` so
    inputs work, hidden on `windowDidResignKey:`, and
    `collectionBehavior |= CanJoinAllSpaces | FullScreenAuxiliary`
  - Windows: `WS_POPUP` + `WS_EX_TOOLWINDOW | WS_EX_TOPMOST`, hidden on
    `WM_ACTIVATE(WA_INACTIVE)`
  - Linux X11: `GTK_WINDOW_POPUP`-like undecorated window with
    `gtk_window_move`, hidden on `focus-out-event`
  - Linux Wayland: the app can't position the window, so it falls back to a
    centered, undecorated, normal window, and Feature `Detail` documents this
    (as with GlobalShortcuts).
- Security tests: the panel has no grants without an explicit one, the
  identity of the panel's invokes is stamped by the host, and navigation
  inside the panel doesn't carry authority.

### M5 — Example, docs, e2e (`docs` / `test`)

- `example/menubar`: an accessory app with a live title (for example, free
  disk space for `$HOME`, using real data from `syscall.Statfs` and polled by
  a Go ticker). It has an HTML panel with meters, a menu with
  Refresh/Settings/Quit, and a `fs.read`-scoped settings window.
- `docs/guide/menubar.md`, `docs/reference/hosts.md` feature-matrix rows for
  `tray.status`, `app.presentation`, `tray.anchor`, `window.panel`.
- Extend `make e2e` (Linux/xvfb): set the tray via SNI, call `Activate` over
  D-Bus, and assert the panel is shown and receives an invoke round-trip.

## Platform matrix (target)

| Feature | macOS | Windows | Linux X11 | Linux Wayland |
|---------|-------|---------|-----------|---------------|
| Tray title | ✓ | tooltip only | SNI label (DE-dependent) | SNI label (DE-dependent) |
| Tray icon / template | ✓ | ✓ (no template) | ✓ | ✓ |
| Accessory mode | ✓ | ✓ | ✓ | ✓ |
| Anchor rect | ✓ | ✓ | ✓ | ✗ (unsupported) |
| Anchored panel | ✓ | ✓ | ✓ | centered fallback |

## Out of scope

- Custom-drawn status item views (live graphs drawn in the menu bar itself).
  Use a rendered PNG icon instead.
- Native menus with embedded HTML/custom views. The panel covers this case.
- Multiple tray icons per app.

### M6 — Launch at login (`feat(desktop)`)

- `platform.LoginItem` interface, Feature `app.login_item`:
  - darwin: `SMAppService.mainApp` (macOS 13+)
  - Windows: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  - Linux: XDG autostart `.desktop` in `$XDG_CONFIG_HOME/autostart`
- Official commands `app.loginItem.get` / `app.loginItem.set`, gated by a new
  `app.login_item` permission.

## Decisions

1. `tray.click` is delivered to the tray panel and the main window only, which
   matches how `tray.action` is delivered today.
2. Launch at login is in scope as M6.
