# Host interfaces

A host is the native side of an app: it opens WebView windows, carries messages between the page and Go, and talks to the OS. Vitra ships one for each OS (`platform/linux`, `platform/darwin`, `platform/windows`). You write your own only for a new platform, an embedded WebView, or tests.

Every interface a host can implement is in package [`platform`](https://pkg.go.dev/go.klarlabs.de/vitra/platform). A host implements the core, plus only the capabilities it supports.

## The core

`platform.DesktopHost` is what `app.Run` needs. `app.Options.Host` takes it (`app.DesktopHost` is an alias).

| Method | What it does |
|---|---|
| `OS()`, `Features()` | The OS family and the feature matrix (see below) |
| `CreateWindow`, `NavigateWindow`, `CloseWindow` | Window lifecycle (`platform.Host`) |
| `PostMessage` | Send a reply or event to a window's page |
| `Open(spec, uri, preload)` | Create a window, inject the preload before any page script, load `uri` |
| `SetInvokeHandler` | Receive page calls. The host stamps the window and origin; it never reads them from the message |
| `SetNavPolicy` | Check every top-level navigation; cancel it when the check returns false |
| `SetDestroyHandler` | Report a window the user closed natively |
| `Run`, `Quit` | The UI loop |

## Optional capabilities

The app checks for each of these with a type assertion when a command needs it. If the host lacks it, the command fails with `*platform.ErrUnsupported` naming the feature. It never succeeds without doing the work.

| Interface | Methods | Used by |
|---|---|---|
| `Clipboard` | `ClipboardGet`, `ClipboardSet` | `clipboard.read`, `clipboard.write` |
| `Dialogs` | `OpenFileDialog`, `SaveFileDialog`, `OpenDirectoryDialog`, `MessageDialog` | `dialog.*` |
| `MultiFileOpener` | `OpenFilesDialog` | `dialog.open` with `multiple: true` |
| `Notifier` | `ShowNotification` | `notifications.show` |
| `MenuBar` | `SetMenuBar`, plus `ActionReporter` | `menu.set`, `menu.clear` |
| `Tray` | `SetTray(TraySpec)`, `ClearTray`, plus `ActionReporter` | `tray.set`, `tray.clear`, `App.SetTray` |
| `TrayClickReporter` | `SetTrayClickHandler` | `tray.click` event, `App.OnTrayClick` (with `TraySpec.ClickActivates`) |
| `TrayAnchorer` | `TrayAnchor` | `App.TrayAnchor` |
| `GlobalShortcuts` | `RegisterGlobalShortcut`, `UnregisterGlobalShortcut`, plus `ActionReporter` | `shortcut.*` |
| `ActionReporter` | `SetActionHandler` | `menu.action`, `tray.action`, `shortcut.action` events |
| `DragDrop` | `EnableDragDrop`, `SetDragDropHandler` | `dragdrop.receive`, `dragdrop.drop` event |
| `WindowControls` | `ApplyWindowChrome`, `ReadWindowChrome`, `FocusWindow`, `BlurWindow` | `window.*` except `create` and `close` |
| `URLOpener` | `OpenURL` | `browser.open` |
| `PathOpener` | `OpenPath` | `path.open` |

These are for your own Go code, not official commands:

| Interface | Methods | For |
|---|---|---|
| `SingleInstance` | `TrySingleInstance`, `StartDeepLinkBridge`, `ForwardToPrimary` | One running instance; handing deep links to it |
| `PresentationSetter` | `SetPresentation` | `app.Options.Presentation` (menu bar apps) |
| `URLSchemeRegistrar` | `RegisterURLScheme` | `vitra register-scheme` |
| `FileAssociationRegistrar` | `RegisterFileAssociations` | `vitra register-files` |
| `ScriptEvaluator` | `Eval` | Tests and automation. Script run this way has the page's bridge access |
| `FileDropInjector` | `InjectFileDrop` | Simulating a file drop in tests and demos |

## Hooks

The app installs these when the host has them. Without them it falls back to the core.

| Interface | Method | Without it |
|---|---|---|
| `DevToolsSetter` | `SetDevTools` | The inspector never opens, even with `Options.DevTools` |
| `MessageReporter` | `SetMessageHandler` | Calls arrive through `SetInvokeHandler`, and the app trusts the origin it last recorded for the window instead of checking each message's sender URL |
| `RejectReporter` | `SetRejectHandler` | Messages the host drops itself are not audited |

The native hosts implement `MessageReporter`; implement it in yours too, so every call's sender is checked.

## Implementing an interface vs. supporting a feature

Implementing an interface says the host has the code. `Features()` says whether it works on this machine, which can depend on the OS version, the desktop session, or a system library. The OS hosts implement every interface in all builds; without `-tags vitra_native` their feature matrix marks everything unavailable. A command needs both, and fails with `ErrUnsupported` if either is missing.

## A minimal host

A host for tests that runs an app and has a clipboard:

```go
type testHost struct {
	// OS, Features, CreateWindow, NavigateWindow, PostMessage, CloseWindow,
	// Open, SetInvokeHandler, SetNavPolicy, SetDestroyHandler, Run, Quit
	core
	text string
}

func (h *testHost) ClipboardGet() (string, error) { return h.text, nil }
func (h *testHost) ClipboardSet(s string) error  { h.text = s; return nil }

var (
	_ platform.DesktopHost = (*testHost)(nil)
	_ platform.Clipboard   = (*testHost)(nil)
)
```

With this host, `clipboard.read` works and `tray.set` fails with `ErrUnsupported` for `tray`. Assert each interface you mean to implement, as above, so a changed signature fails to compile instead of silently dropping the capability.
