package platform

import (
	"context"

	"go.klarlabs.de/vitra/domain"
)

// This file lists every interface a desktop host can implement.
//
// DesktopHost is the core: everything app.Run needs to serve a page, route
// its calls and run the UI loop. Every other interface is optional. The app
// detects each one with a type assertion, and a command that needs a missing
// capability fails with *ErrUnsupported naming the feature. It never succeeds
// silently (security invariant 14). A custom or test host implements the core
// and only the capabilities it supports.
//
// The native hosts in platform/darwin, platform/linux and platform/windows
// implement every interface here (MessageReporter and RejectReporter only
// with -tags vitra_native), and still report per-feature support through
// Features: implementing an interface says the host has the code, Features
// says whether it works on this machine.

// DesktopHost is the core every host implements to run a Vitra app: open and
// close WebView windows, deliver page calls to the app, enforce its
// navigation policy, and run the UI loop. Optional capabilities are separate
// interfaces in this package.
type DesktopHost interface {
	Host
	// Open creates window spec, injects preload before any page script runs,
	// and loads uri.
	Open(spec WindowSpec, uri, preload string) error
	// SetInvokeHandler installs the handler for page bridge messages. The
	// host stamps the window and origin; it never takes them from the payload.
	// The returned bytes, if any, are the reply to post back to the page.
	SetInvokeHandler(fn func(windowID domain.WindowID, origin domain.Origin, raw []byte) []byte)
	// SetNavPolicy installs the check every top-level navigation must pass.
	// The host cancels the navigation when fn returns false.
	SetNavPolicy(fn func(windowID domain.WindowID, uri string) bool)
	// SetDestroyHandler installs the callback for a window the user closed
	// natively (for example from the title bar).
	SetDestroyHandler(fn func(windowID domain.WindowID))
	// Run blocks on the UI loop until Quit.
	Run() error
	// Quit ends the UI loop.
	Quit()
}

// Hooks the app installs when the host has them. Without them the app falls
// back to the core behaviour described on each.

// DevToolsSetter is implemented by hosts whose WebView inspector can be
// toggled. The app calls it before any window opens. A host without it never
// opens the inspector.
type DevToolsSetter interface {
	SetDevTools(enabled bool)
}

// MessageReporter is implemented by hosts that report the URL of the document
// that sent each bridge message. The app then checks the sender of every
// message instead of trusting the origin it last recorded for the window.
// Without it, calls arrive through DesktopHost.SetInvokeHandler.
type MessageReporter interface {
	SetMessageHandler(fn func(windowID domain.WindowID, senderURL string, raw []byte) []byte)
}

// RejectReporter is implemented by hosts that drop some bridge messages
// before they reach the app (macOS drops messages posted by subframes), so
// the app can audit them like the messages it drops itself.
type RejectReporter interface {
	SetRejectHandler(fn func(windowID domain.WindowID, reason string))
}

// ActionReporter delivers native activations: menu items, tray items and
// global shortcuts, by their action ID. MenuBar, Tray and GlobalShortcuts
// include it, so a host that shows an actionable item also reports it.
type ActionReporter interface {
	SetActionHandler(fn func(id string))
}

// Optional capabilities, each backing one or more official commands.

// Clipboard reads and writes plain text on the system clipboard
// (FeatureClipboard).
type Clipboard interface {
	ClipboardGet() (string, error)
	ClipboardSet(text string) error
}

// Dialogs shows native file and message dialogs (FeatureDialogOpen,
// FeatureDialogSave, FeatureDialogOpenDirectory, FeatureDialogMessage). The
// path-returning dialogs return "" when the user cancels. Choosing a path
// grants nothing: reading or writing it still needs an fs grant.
type Dialogs interface {
	OpenFileDialog(opts DialogFileOptions) (string, error)
	SaveFileDialog(opts DialogFileOptions) (string, error)
	OpenDirectoryDialog(opts DialogFileOptions) (string, error)
	// MessageDialog shows a message of kind "info", "warning", "error" or
	// "question", and reports whether the user confirmed it.
	MessageDialog(title, message, kind string) (bool, error)
}

// MultiFileOpener is implemented by hosts whose open-file dialog can select
// several files at once (DialogFileOptions.Multiple). A caller asking for
// multiple files from a host without it gets an explicit ErrUnsupported,
// never a silent single selection.
//
// OpenFilesDialog returns every selected path, or nil when the user cancels.
// Selecting a file grants nothing: reading it still needs an fs grant.
type MultiFileOpener interface {
	OpenFilesDialog(opts DialogFileOptions) ([]string, error)
}

// Notifier shows desktop notifications (FeatureNotificationShow).
type Notifier interface {
	ShowNotification(title, body string) error
}

// MenuBar sets a window's native menu bar (FeatureMenuBar). Nil items clear
// it. Activations are reported through ActionReporter.
type MenuBar interface {
	ActionReporter
	SetMenuBar(id domain.WindowID, items []MenuItem) error
}

// Tray shows a system tray icon with a menu (FeatureTray). Activations are
// reported through ActionReporter.
type Tray interface {
	ActionReporter
	SetTray(spec TraySpec) error
	ClearTray()
}

// GlobalShortcuts registers system-wide hotkeys (FeatureGlobalShortcut).
// Activations are reported through ActionReporter with the registered
// action ID.
type GlobalShortcuts interface {
	ActionReporter
	RegisterGlobalShortcut(accelerator, actionID string) error
	UnregisterGlobalShortcut(accelerator string) error
}

// DragDrop delivers files dropped on a window (FeatureDragDrop). Windows
// accept drops only after EnableDragDrop.
type DragDrop interface {
	EnableDragDrop(id domain.WindowID, enabled bool) error
	SetDragDropHandler(fn func(windowID domain.WindowID, paths []string))
}

// WindowControls changes and reads a window's native presentation: title,
// size, state, icon and focus (FeatureWindowChrome).
type WindowControls interface {
	ApplyWindowChrome(id domain.WindowID, chrome WindowChrome) error
	ReadWindowChrome(id domain.WindowID) (WindowChrome, error)
	FocusWindow(id domain.WindowID) error
	BlurWindow(id domain.WindowID) error
}

// URLOpener opens a URL in the default browser (FeatureOpenURL).
type URLOpener interface {
	OpenURL(ctx context.Context, rawURL string) error
}

// PathOpener opens a file or folder with its default application
// (FeaturePathOpen).
type PathOpener interface {
	OpenPath(ctx context.Context, path string) error
}

// Lifecycle and OS integration, used by the app itself rather than by
// official commands.

// SingleInstance keeps one running instance per app ID and hands deep links
// from later launches to it (FeatureSingleInstance, FeatureDeepLink).
type SingleInstance interface {
	// TrySingleInstance takes the app-wide lock. held is false when another
	// instance has it; call release when done.
	TrySingleInstance(appID string) (held bool, release func(), err error)
	// StartDeepLinkBridge listens, in the primary instance, for links
	// forwarded by later launches.
	StartDeepLinkBridge(appID string, onLink func(raw string)) (stop func(), err error)
	// ForwardToPrimary sends urls to the running primary instance.
	ForwardToPrimary(appID string, urls []string) (ok bool, err error)
}

// Presentation is how the app appears in the OS shell.
type Presentation string

const (
	// PresentationRegular is a normal app: a Dock icon on macOS and a
	// taskbar entry for each window elsewhere.
	PresentationRegular Presentation = "regular"
	// PresentationAccessory is a menu bar or tray app: no Dock icon on
	// macOS, and no taskbar or pager entries for its windows elsewhere.
	PresentationAccessory Presentation = "accessory"
)

// PresentationSetter switches the app between a regular and an accessory
// presentation (FeaturePresentation). It applies to windows already open
// and to every window opened later.
type PresentationSetter interface {
	SetPresentation(p Presentation) error
}

// URLSchemeRegistrar registers the app as the handler of a URL scheme
// (FeatureDeepLink).
type URLSchemeRegistrar interface {
	RegisterURLScheme(scheme, appID, execPath string) error
}

// FileAssociationRegistrar registers the app as a handler for MIME types
// (FeatureFileAssociation).
type FileAssociationRegistrar interface {
	RegisterFileAssociations(appID, execPath, name string, mimeTypes []string) error
}

// Test and automation hooks.

// ScriptEvaluator runs script in a window's page. It is for tests and
// automation: script run this way has the page's bridge access.
type ScriptEvaluator interface {
	Eval(id domain.WindowID, js string) error
}

// FileDropInjector simulates a native file drop on a window, for tests and
// demos. It delivers to the DragDrop handler as a real drop would.
type FileDropInjector interface {
	InjectFileDrop(id domain.WindowID, paths []string)
}
