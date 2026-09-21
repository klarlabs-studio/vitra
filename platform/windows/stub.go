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

func (h *Host) OS() platform.OS { return platform.OSWindows }

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

func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }
func (h *Host) CreateWindow(context.Context, platform.WindowSpec) error {
	return h.err(platform.FeatureWindowCreate)
}
func (h *Host) Open(platform.WindowSpec, string, string) error {
	return h.err(platform.FeatureWindowCreate)
}
func (h *Host) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return h.err(platform.FeatureWindowNavigate)
}
func (h *Host) PostMessage(context.Context, domain.WindowID, []byte) error {
	return h.err(platform.FeatureWebViewMessage)
}
func (h *Host) Eval(domain.WindowID, string) error { return h.err(platform.FeatureWebViewMessage) }
func (h *Host) CloseWindow(context.Context, domain.WindowID) error {
	return h.err(platform.FeatureWindowCreate)
}
func (h *Host) OpenFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogOpen)
}
func (h *Host) SaveFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogSave)
}
func (h *Host) MessageDialog(string, string, string) (bool, error) {
	return false, h.err(platform.FeatureDialogMessage)
}
func (h *Host) ShowNotification(string, string) error {
	return h.err(platform.FeatureNotificationShow)
}
func (h *Host) SetActionHandler(func(string))                      {}
func (h *Host) SetDragDropHandler(func(domain.WindowID, []string)) {}
func (h *Host) SetDestroyHandler(func(domain.WindowID))            {}
func (h *Host) EnableDragDrop(domain.WindowID, bool) error {
	return h.err(platform.FeatureDragDrop)
}
func (h *Host) InjectFileDrop(domain.WindowID, []string) {}
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}
func (h *Host) SetTray(string, []platform.MenuItem) error {
	return h.err(platform.FeatureTray)
}
func (h *Host) ClearTray() {}
func (h *Host) RegisterGlobalShortcut(string, string) error {
	return h.err(platform.FeatureGlobalShortcut)
}
func (h *Host) UnregisterGlobalShortcut(string) error {
	return h.err(platform.FeatureGlobalShortcut)
}
func (h *Host) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error {
	return h.err(platform.FeatureWindowChrome)
}
func (h *Host) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, h.err(platform.FeatureWindowChrome)
}
func (h *Host) Run() error { return h.err(platform.FeatureWindowCreate) }
func (h *Host) Quit()      {}

func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSWindows,
		Detail:  "requires CGO_ENABLED=1 -tags vitra_native (Win32 + WebView2 DesktopHost)",
	}
}
