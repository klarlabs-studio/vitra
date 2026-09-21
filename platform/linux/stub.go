//go:build !linux || !cgo || !vitra_native

package linux

import (
	"context"
	"errors"

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

func (h *Host) OS() platform.OS { return platform.OSLinux }
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
func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }
func (h *Host) SetActionHandler(fn func(id string))                { h.onAction = fn }
func (h *Host) SetDragDropHandler(fn func(domain.WindowID, []string)) {
	h.onDrop = fn
}
func (h *Host) SetDestroyHandler(fn func(domain.WindowID)) {
	h.onDestroy = fn
}
func (h *Host) EnableDragDrop(domain.WindowID, bool) error { return h.err() }
func (h *Host) InjectFileDrop(id domain.WindowID, paths []string) {
	if h.onDrop != nil {
		h.onDrop(id, append([]string(nil), paths...))
	}
}
func (h *Host) ActivateMenuAccel(domain.WindowID, string) (bool, error) {
	return false, h.err()
}
func (h *Host) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error { return h.err() }
func (h *Host) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, h.err()
}
func (h *Host) CreateWindow(context.Context, platform.WindowSpec) error {
	return h.err()
}
func (h *Host) Open(platform.WindowSpec, string, string) error { return h.err() }
func (h *Host) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return h.err()
}
func (h *Host) PostMessage(context.Context, domain.WindowID, []byte) error { return h.err() }
func (h *Host) Eval(domain.WindowID, string) error                         { return h.err() }
func (h *Host) CloseWindow(context.Context, domain.WindowID) error         { return h.err() }
func (h *Host) ClipboardGet() (string, error)                              { return "", h.err() }
func (h *Host) ClipboardSet(string) error                                  { return h.err() }
func (h *Host) OpenFileDialog(platform.DialogFileOptions) (string, error)  { return "", h.err() }
func (h *Host) SaveFileDialog(platform.DialogFileOptions) (string, error)  { return "", h.err() }
func (h *Host) OpenDirectoryDialog() (string, error)                       { return "", h.err() }
func (h *Host) MessageDialog(string, string, string) (bool, error)         { return false, h.err() }
func (h *Host) ShowNotification(string, string) error                      { return h.err() }
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error      { return h.err() }
func (h *Host) SetTray(string, []platform.MenuItem) error                  { return h.err() }
func (h *Host) ClearTray()                                                 {}
func (h *Host) RegisterGlobalShortcut(string, string) error                { return h.err() }
func (h *Host) UnregisterGlobalShortcut(string) error                      { return h.err() }
func (h *Host) Run() error                                                 { return h.err() }
func (h *Host) Quit()                                                      {}
func (h *Host) err() error {
	return errors.New("linux webview host requires CGO_ENABLED=1 -tags vitra_native and webkit2gtk-4.1")
}

// MenuItem matches the portable chrome menu entry.
type MenuItem = platform.MenuItem
