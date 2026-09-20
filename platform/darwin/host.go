// Package darwin provides the macOS WKWebView host adapter.
//
// The competitive Linux host is production-linked today. Darwin follows the
// same DesktopHost surface; this package ships an explicit FeatureSet so
// callers never see silent no-ops (security invariant 14).
package darwin

import (
	"context"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Host is the macOS desktop adapter placeholder.
type Host struct {
	onInvoke func(domain.WindowID, domain.Origin, []byte) []byte
	onNav    func(domain.WindowID, string) bool
}

// New returns a Darwin host. Native WKWebView linking is the next platform milestone.
func New() *Host { return &Host{} }

func (h *Host) OS() platform.OS { return platform.OSDarwin }

func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {
			Feature: platform.FeatureWindowCreate, Available: false,
			Detail: "WKWebView adapter not yet linked; API-compatible stub",
		},
		platform.FeatureWindowNavigate: {
			Feature: platform.FeatureWindowNavigate, Available: false,
			Detail: "WKWebView adapter not yet linked",
		},
		platform.FeatureWebViewMessage: {
			Feature: platform.FeatureWebViewMessage, Available: false,
			Detail: "WKWebView adapter not yet linked",
		},
		platform.FeatureClipboard: {
			Feature: platform.FeatureClipboard, Available: false,
			Detail: "NSPasteboard wiring pending native host",
		},
		platform.FeatureDialogOpen: {
			Feature: platform.FeatureDialogOpen, Available: false,
			Detail: "NSOpenPanel wiring pending native host",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: false,
			Detail: "NSMenu pending native host",
		},
		platform.FeatureDialogSave: {
			Feature: platform.FeatureDialogSave, Available: false,
			Detail: "save dialog pending native host",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: false,
			Detail: "single-instance lock pending native host",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "global shortcuts pending native host",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: false,
			Detail: "deep-link argv/socket handoff pending native host",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: false,
			Detail: "NSStatusItem pending native host",
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
func (h *Host) ClipboardGet() (string, error) {
	return "", h.err(platform.FeatureClipboard)
}
func (h *Host) ClipboardSet(string) error { return h.err(platform.FeatureClipboard) }
func (h *Host) OpenFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogOpen)
}
func (h *Host) SaveFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogSave)
}
func (h *Host) SetActionHandler(func(string)) {}
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}
func (h *Host) SetTray(string) error { return h.err(platform.FeatureTray) }
func (h *Host) ClearTray()           {}
func (h *Host) TrySingleInstance(string) (bool, func(), error) {
	return false, nil, h.err(platform.FeatureSingleInstance)
}
func (h *Host) StartDeepLinkBridge(string, func(string)) (func(), error) {
	return nil, h.err(platform.FeatureDeepLink)
}
func (h *Host) ForwardToPrimary(string, []string) (bool, error) {
	return false, h.err(platform.FeatureDeepLink)
}
func (h *Host) Run() error { return h.err(platform.FeatureWindowCreate) }
func (h *Host) Quit()      {}

func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSDarwin,
		Detail:  "WKWebView host not yet linked; use platform/linux with -tags vitra_native on Linux",
	}
}
