//go:build !linux || !cgo || !vitra_native

// Package linux provides the Linux WebKitGTK/GTK3 host adapter.
//
// Without CGO_ENABLED=1 -tags vitra_native this is an explicit stub
// (security invariant 14). Enable the native host on Linux with:
//
//	CGO_ENABLED=1 go build -tags vitra_native
package linux

import (
	"context"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Host is a stub when the native WebKitGTK host is not linked.
// Enable with: CGO_ENABLED=1 go build -tags vitra_native
type Host struct {
	onInvoke  func(domain.WindowID, domain.Origin, []byte) []byte
	onNav     func(domain.WindowID, string) bool
	onAction  func(id string)
	onDrop    func(windowID domain.WindowID, paths []string)
	onDestroy func(windowID domain.WindowID)
}

// New returns a stub host that reports why native UI is unavailable.
func New() *Host { return &Host{} }

// SetProgramName is a no-op on the stub host.
func (h *Host) SetProgramName(string) {}

// ProgramName returns empty on the stub host.
func (h *Host) ProgramName() string { return "" }

// OS reports the operating system this host targets.
func (h *Host) OS() platform.OS { return platform.OSLinux }

// Features reports which features this build supports, and why the rest are unavailable.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {
			Feature: platform.FeatureWindowCreate, Available: false,
			Detail: "requires CGO_ENABLED=1 -tags vitra_native and webkit2gtk-4.1",
		},
		platform.FeatureClipboard: {
			Feature: platform.FeatureClipboard, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureDialogOpen: {
			Feature: platform.FeatureDialogOpen, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureDialogSave: {
			Feature: platform.FeatureDialogSave, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureDialogOpenDirectory: {
			Feature: platform.FeatureDialogOpenDirectory, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureDialogMessage: {
			Feature: platform.FeatureDialogMessage, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureNotificationShow: {
			Feature: platform.FeatureNotificationShow, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: true,
			Detail: "flock-based; available without native WebView",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: true,
			Detail: "argv + socket handoff + xdg URL-scheme registration",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureFileAssociation: {
			Feature: platform.FeatureFileAssociation, Available: true,
			Detail: "xdg MIME desktop file; available without native WebView",
		},
		platform.FeatureWindowChrome: {
			Feature: platform.FeatureWindowChrome, Available: false,
			Detail: "requires native linux host",
		},
		platform.FeatureOpenURL: {
			Feature: platform.FeatureOpenURL, Available: true,
			Detail: "xdg-open for http(s)/mailto; available without native WebView",
		},
		platform.FeaturePathOpen: {
			Feature: platform.FeaturePathOpen, Available: true,
			Detail: "open absolute local paths with OS default handler",
		},
	}
}

// SetInvokeHandler stores the callback; without the native host it is never called.
func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}

// SetNavPolicy stores the callback; without the native host it is never called.
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }

// SetActionHandler stores the callback; without the native host it is never called.
func (h *Host) SetActionHandler(fn func(id string)) { h.onAction = fn }

// SetDragDropHandler stores the callback; without the native host it is never called.
func (h *Host) SetDragDropHandler(fn func(domain.WindowID, []string)) {
	h.onDrop = fn
}

// SetDestroyHandler stores the callback; without the native host it is never called.
func (h *Host) SetDestroyHandler(fn func(domain.WindowID)) {
	h.onDestroy = fn
}

// EnableDragDrop returns ErrUnsupported: it needs the native host.
func (h *Host) EnableDragDrop(domain.WindowID, bool) error { return h.err(platform.FeatureDragDrop) }

// InjectFileDrop passes paths to the drag-drop handler, as a native drop would.
func (h *Host) InjectFileDrop(id domain.WindowID, paths []string) {
	if h.onDrop != nil {
		h.onDrop(id, append([]string(nil), paths...))
	}
}

// ActivateMenuAccel returns ErrUnsupported: it needs the native host.
func (h *Host) ActivateMenuAccel(domain.WindowID, string) (bool, error) {
	return false, h.err(platform.FeatureMenuBar)
}

// ApplyWindowChrome returns ErrUnsupported: it needs the native host.
func (h *Host) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error {
	return h.err(platform.FeatureWindowChrome)
}

// ReadWindowChrome returns ErrUnsupported: it needs the native host.
func (h *Host) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, h.err(platform.FeatureWindowChrome)
}

// FocusWindow returns ErrUnsupported: it needs the native host.
func (h *Host) FocusWindow(domain.WindowID) error { return h.err(platform.FeatureWindowChrome) }

// BlurWindow returns ErrUnsupported: it needs the native host.
func (h *Host) BlurWindow(domain.WindowID) error { return h.err(platform.FeatureWindowChrome) }

// CreateWindow returns ErrUnsupported: it needs the native host.
func (h *Host) CreateWindow(context.Context, platform.WindowSpec) error {
	return h.err(platform.FeatureWindowCreate)
}

// Open returns ErrUnsupported: it needs the native host.
func (h *Host) Open(platform.WindowSpec, string, string) error {
	return h.err(platform.FeatureWindowCreate)
}

// NavigateWindow returns ErrUnsupported: it needs the native host.
func (h *Host) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return h.err(platform.FeatureWindowNavigate)
}

// PostMessage returns ErrUnsupported: it needs the native host.
func (h *Host) PostMessage(context.Context, domain.WindowID, []byte) error {
	return h.err(platform.FeatureWebViewMessage)
}

// Eval returns ErrUnsupported: it needs the native host.
func (h *Host) Eval(domain.WindowID, string) error { return h.err(platform.FeatureWebViewMessage) }

// CloseWindow returns ErrUnsupported: it needs the native host.
func (h *Host) CloseWindow(context.Context, domain.WindowID) error {
	return h.err(platform.FeatureWindowCreate)
}

// ClipboardGet returns ErrUnsupported: it needs the native host.
func (h *Host) ClipboardGet() (string, error) { return "", h.err(platform.FeatureClipboard) }

// ClipboardSet returns ErrUnsupported: it needs the native host.
func (h *Host) ClipboardSet(string) error { return h.err(platform.FeatureClipboard) }

// OpenFileDialog returns ErrUnsupported: it needs the native host.
func (h *Host) OpenFileDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogOpen)
}

// The stub keeps the native host's method set; it still fails explicitly.
var _ platform.MultiFileOpener = (*Host)(nil)

// OpenFilesDialog returns ErrUnsupported: it needs the native host.
func (h *Host) OpenFilesDialog(platform.DialogFileOptions) ([]string, error) {
	return nil, h.err(platform.FeatureDialogOpen)
}

// SaveFileDialog returns ErrUnsupported: it needs the native host.
func (h *Host) SaveFileDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogSave)
}

// OpenDirectoryDialog returns ErrUnsupported: it needs the native host.
func (h *Host) OpenDirectoryDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogOpenDirectory)
}

// MessageDialog returns ErrUnsupported: it needs the native host.
func (h *Host) MessageDialog(string, string, string) (bool, error) {
	return false, h.err(platform.FeatureDialogMessage)
}

// ShowNotification returns ErrUnsupported: it needs the native host.
func (h *Host) ShowNotification(string, string) error { return h.err(platform.FeatureNotificationShow) }

// SetMenuBar returns ErrUnsupported: it needs the native host.
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}

// SetTray returns ErrUnsupported: it needs the native host.
func (h *Host) SetTray(platform.TraySpec) error { return h.err(platform.FeatureTray) }

// SetPresentation returns ErrUnsupported (FeaturePresentation): it needs
// the native host.
func (h *Host) SetPresentation(platform.Presentation) error {
	return h.err(platform.FeaturePresentation)
}

// SetTrayClickHandler does nothing without the native host.
func (h *Host) SetTrayClickHandler(func(platform.TrayClick)) {}

// TrayAnchor returns ErrUnsupported (FeatureTrayAnchor): it needs the
// native host.
func (h *Host) TrayAnchor() (platform.Rect, error) {
	return platform.Rect{}, h.err(platform.FeatureTrayAnchor)
}

// ShowPanel returns ErrUnsupported (FeatureWindowPanel): it needs the
// native host.
func (h *Host) ShowPanel(domain.WindowID, platform.Rect, bool) error {
	return h.err(platform.FeatureWindowPanel)
}

// HidePanel returns ErrUnsupported (FeatureWindowPanel).
func (h *Host) HidePanel(domain.WindowID) error { return h.err(platform.FeatureWindowPanel) }

// PanelShown returns ErrUnsupported (FeatureWindowPanel).
func (h *Host) PanelShown(domain.WindowID) (bool, error) {
	return false, h.err(platform.FeatureWindowPanel)
}

// ClearTray does nothing without the native host.
func (h *Host) ClearTray() {}

// RegisterGlobalShortcut returns ErrUnsupported: it needs the native host.
func (h *Host) RegisterGlobalShortcut(string, string) error {
	return h.err(platform.FeatureGlobalShortcut)
}

// UnregisterGlobalShortcut returns ErrUnsupported: it needs the native host.
func (h *Host) UnregisterGlobalShortcut(string) error { return h.err(platform.FeatureGlobalShortcut) }

// Run returns ErrUnsupported: it needs the native host.
func (h *Host) Run() error { return h.err(platform.FeatureWindowCreate) }

// Quit does nothing without the native host.
func (h *Host) Quit() {}
func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSLinux,
		Detail:  "requires CGO_ENABLED=1 -tags vitra_native and webkit2gtk-4.1 (WebKitGTK DesktopHost)",
	}
}

// MenuItem matches the portable chrome menu entry.
type MenuItem = platform.MenuItem

// SetDevTools is a no-op without the native host.
func (h *Host) SetDevTools(bool) {}
