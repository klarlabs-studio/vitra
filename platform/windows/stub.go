//go:build !windows || !cgo || !vitra_native

// Package windows provides the Windows WebView2 host adapter.
//
// Without CGO_ENABLED=1 -tags vitra_native this is an explicit stub
// (security invariant 14). Enable the native host on Windows with:
//
//	CGO_ENABLED=1 go build -tags vitra_native
package windows

import (
	"context"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Host is the Windows desktop adapter placeholder.
type Host struct {
	onInvoke func(domain.WindowID, domain.Origin, []byte) []byte
	onNav    func(domain.WindowID, string) bool
}

// New returns a stub host that reports why native UI is unavailable.
func New() *Host { return &Host{} }

// SetProgramName is a no-op on the stub host.
func (h *Host) SetProgramName(string) {}

// ProgramName returns empty on the stub host.
func (h *Host) ProgramName() string { return "" }

// OS reports the operating system this host targets.
func (h *Host) OS() platform.OS { return platform.OSWindows }

// Features reports which features this build supports, and why the rest are unavailable.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {
			Feature: platform.FeatureWindowCreate, Available: false,
			Detail: "requires CGO_ENABLED=1 -tags vitra_native on Windows (Win32 + WebView2)",
		},
		platform.FeatureWindowNavigate: {
			Feature: platform.FeatureWindowNavigate, Available: false,
			Detail: "requires CGO_ENABLED=1 -tags vitra_native + WebView2Loader.dll",
		},
		platform.FeatureWebViewMessage: {
			Feature: platform.FeatureWebViewMessage, Available: false,
			Detail: "requires CGO_ENABLED=1 -tags vitra_native + WebView2Loader.dll",
		},
		platform.FeatureClipboard: {
			Feature: platform.FeatureClipboard, Available: true,
			Detail: "PowerShell Get/Set-Clipboard; available without native WebView",
		},
		platform.FeatureDialogOpen: {
			Feature: platform.FeatureDialogOpen, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureDialogSave: {
			Feature: platform.FeatureDialogSave, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureDialogOpenDirectory: {
			Feature: platform.FeatureDialogOpenDirectory, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureDialogMessage: {
			Feature: platform.FeatureDialogMessage, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureNotificationShow: {
			Feature: platform.FeatureNotificationShow, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: true,
			Detail: "exclusive lock file; available without native WebView",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: true,
			Detail: "argv + socket handoff + HKCU Classes .reg URL-scheme registration",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureWindowChrome: {
			Feature: platform.FeatureWindowChrome, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureOpenURL: {
			Feature: platform.FeatureOpenURL, Available: true,
			Detail: "cmd start for http(s)/mailto; available without native WebView",
		},
		platform.FeaturePathOpen: {
			Feature: platform.FeaturePathOpen, Available: true,
			Detail: "open absolute local paths with OS default handler",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: false,
			Detail: "requires native windows host",
		},
		platform.FeatureFileAssociation: {
			Feature: platform.FeatureFileAssociation, Available: true,
			Detail: "HKCU ProgID + MIME .reg; available without native WebView",
		},
	}
}

// SetInvokeHandler stores the callback; without the native host it is never called.
func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}

// SetNavPolicy stores the callback; without the native host it is never called.
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }

// CreateWindow returns ErrUnsupported (FeatureWindowCreate): it needs the native host.
func (h *Host) CreateWindow(context.Context, platform.WindowSpec) error {
	return h.err(platform.FeatureWindowCreate)
}

// Open returns ErrUnsupported (FeatureWindowCreate): it needs the native host.
func (h *Host) Open(platform.WindowSpec, string, string) error {
	return h.err(platform.FeatureWindowCreate)
}

// NavigateWindow returns ErrUnsupported (FeatureWindowNavigate): it needs the native host.
func (h *Host) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return h.err(platform.FeatureWindowNavigate)
}

// PostMessage returns ErrUnsupported (FeatureWebViewMessage): it needs the native host.
func (h *Host) PostMessage(context.Context, domain.WindowID, []byte) error {
	return h.err(platform.FeatureWebViewMessage)
}

// Eval returns ErrUnsupported (FeatureWebViewMessage): it needs the native host.
func (h *Host) Eval(domain.WindowID, string) error { return h.err(platform.FeatureWebViewMessage) }

// CloseWindow returns ErrUnsupported (FeatureWindowCreate): it needs the native host.
func (h *Host) CloseWindow(context.Context, domain.WindowID) error {
	return h.err(platform.FeatureWindowCreate)
}

// OpenFileDialog returns ErrUnsupported (FeatureDialogOpen): it needs the native host.
func (h *Host) OpenFileDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogOpen)
}

// The stub keeps the native host's method set; it still fails explicitly.
var _ platform.MultiFileOpener = (*Host)(nil)

// OpenFilesDialog returns ErrUnsupported (FeatureDialogOpen): it needs the native host.
func (h *Host) OpenFilesDialog(platform.DialogFileOptions) ([]string, error) {
	return nil, h.err(platform.FeatureDialogOpen)
}

// SaveFileDialog returns ErrUnsupported (FeatureDialogSave): it needs the native host.
func (h *Host) SaveFileDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogSave)
}

// OpenDirectoryDialog returns ErrUnsupported (FeatureDialogOpenDirectory): it needs the native host.
func (h *Host) OpenDirectoryDialog(platform.DialogFileOptions) (string, error) {
	return "", h.err(platform.FeatureDialogOpenDirectory)
}

// MessageDialog returns ErrUnsupported (FeatureDialogMessage): it needs the native host.
func (h *Host) MessageDialog(string, string, string) (bool, error) {
	return false, h.err(platform.FeatureDialogMessage)
}

// ShowNotification returns ErrUnsupported (FeatureNotificationShow): it needs the native host.
func (h *Host) ShowNotification(string, string) error {
	return h.err(platform.FeatureNotificationShow)
}

// SetActionHandler does nothing without the native host.
func (h *Host) SetActionHandler(func(string)) {}

// SetDragDropHandler does nothing without the native host.
func (h *Host) SetDragDropHandler(func(domain.WindowID, []string)) {}

// SetDestroyHandler does nothing without the native host.
func (h *Host) SetDestroyHandler(func(domain.WindowID)) {}

// EnableDragDrop returns ErrUnsupported (FeatureDragDrop): it needs the native host.
func (h *Host) EnableDragDrop(domain.WindowID, bool) error {
	return h.err(platform.FeatureDragDrop)
}

// InjectFileDrop passes paths to the drag-drop handler, as a native drop would.
func (h *Host) InjectFileDrop(domain.WindowID, []string) {}

// SetMenuBar returns ErrUnsupported (FeatureMenuBar): it needs the native host.
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}

// SetTray returns ErrUnsupported (FeatureTray): it needs the native host.
func (h *Host) SetTray(platform.TraySpec) error {
	return h.err(platform.FeatureTray)
}

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

// RegisterGlobalShortcut returns ErrUnsupported (FeatureGlobalShortcut): it needs the native host.
func (h *Host) RegisterGlobalShortcut(string, string) error {
	return h.err(platform.FeatureGlobalShortcut)
}

// UnregisterGlobalShortcut returns ErrUnsupported (FeatureGlobalShortcut): it needs the native host.
func (h *Host) UnregisterGlobalShortcut(string) error {
	return h.err(platform.FeatureGlobalShortcut)
}

// ApplyWindowChrome returns ErrUnsupported (FeatureWindowChrome): it needs the native host.
func (h *Host) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error {
	return h.err(platform.FeatureWindowChrome)
}

// ReadWindowChrome returns ErrUnsupported (FeatureWindowChrome): it needs the native host.
func (h *Host) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, h.err(platform.FeatureWindowChrome)
}

// FocusWindow returns ErrUnsupported (FeatureWindowChrome): it needs the native host.
func (h *Host) FocusWindow(domain.WindowID) error {
	return h.err(platform.FeatureWindowChrome)
}

// BlurWindow returns ErrUnsupported (FeatureWindowChrome): it needs the native host.
func (h *Host) BlurWindow(domain.WindowID) error {
	return h.err(platform.FeatureWindowChrome)
}

// Run returns ErrUnsupported (FeatureWindowCreate): it needs the native host.
func (h *Host) Run() error { return h.err(platform.FeatureWindowCreate) }

// Quit does nothing without the native host.
func (h *Host) Quit() {}

func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSWindows,
		Detail:  "requires CGO_ENABLED=1 -tags vitra_native (Win32 + WebView2 DesktopHost)",
	}
}

// SetDevTools is a no-op without the native host.
func (h *Host) SetDevTools(bool) {}
