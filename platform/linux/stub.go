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
	onInvoke func(domain.WindowID, domain.Origin, []byte) []byte
	onNav    func(domain.WindowID, string) bool
	onAction func(id string)
}

// New returns a stub host that reports why native UI is unavailable.
func New() *Host { return &Host{} }

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
	}
}
func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }
func (h *Host) SetActionHandler(fn func(id string))                { h.onAction = fn }
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
func (h *Host) OpenFileDialog() (string, error)                            { return "", h.err() }
func (h *Host) SaveFileDialog() (string, error)                            { return "", h.err() }
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error      { return h.err() }
func (h *Host) SetTray(string, []platform.MenuItem) error                  { return h.err() }
func (h *Host) ClearTray()                                                 {}
func (h *Host) Run() error                                                 { return h.err() }
func (h *Host) Quit()                                                      {}
func (h *Host) err() error {
	return errors.New("linux webview host requires CGO_ENABLED=1 -tags vitra_native and webkit2gtk-4.1")
}

// MenuItem matches the portable chrome menu entry.
type MenuItem = platform.MenuItem
