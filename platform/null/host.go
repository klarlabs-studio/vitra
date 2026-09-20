// Package null provides a Phase 0 stub platform host.
//
// It records intent and returns explicit ErrUnsupported for native operations
// that require a real WebView adapter — proving the contract that unsupported
// behaviour is never a silent no-op.
package null

import (
	"context"
	"sync"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Host is an in-process stub adapter used in spikes and tests.
type Host struct {
	mu       sync.Mutex
	os       platform.OS
	features platform.FeatureSet
	windows  map[domain.WindowID]domain.Origin
	messages []PostedMessage
}

// PostedMessage records a PostMessage call for spike inspection.
type PostedMessage struct {
	Window  domain.WindowID
	Message []byte
}

// New constructs a null host for the given OS label.
// Message IPC is available (in-process); native window chrome is not.
func New(os platform.OS) *Host {
	return &Host{
		os: os,
		features: platform.FeatureSet{
			platform.FeatureWindowCreate:   {Feature: platform.FeatureWindowCreate, Available: true, Detail: "in-process stub window"},
			platform.FeatureWindowNavigate: {Feature: platform.FeatureWindowNavigate, Available: true},
			platform.FeatureWebViewMessage: {Feature: platform.FeatureWebViewMessage, Available: true, Detail: "in-process message bus"},
			platform.FeatureMenuBar:        {Feature: platform.FeatureMenuBar, Available: false, Detail: "Phase 0 spike: no native menu"},
			platform.FeatureTray:           {Feature: platform.FeatureTray, Available: false, Detail: "Phase 0 spike: no tray"},
			platform.FeatureDialogOpen:     {Feature: platform.FeatureDialogOpen, Available: false, Detail: "Phase 0 spike: no native dialog"},
			platform.FeatureClipboard:      {Feature: platform.FeatureClipboard, Available: false, Detail: "Phase 0 spike: no clipboard"},
		},
		windows: make(map[domain.WindowID]domain.Origin),
	}
}

// OS returns the configured OS label.
func (h *Host) OS() platform.OS { return h.os }

// Features returns the support matrix.
func (h *Host) Features() platform.FeatureSet { return h.features }

// CreateWindow records a stub window.
func (h *Host) CreateWindow(_ context.Context, spec platform.WindowSpec) error {
	if err := platform.Require(h, platform.FeatureWindowCreate); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.windows[spec.ID] = spec.Origin
	return nil
}

// NavigateWindow updates the stub window origin.
func (h *Host) NavigateWindow(_ context.Context, id domain.WindowID, origin domain.Origin) error {
	if err := platform.Require(h, platform.FeatureWindowNavigate); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.windows[id]; !ok {
		return &domain.ErrNotFound{Entity: "window", ID: string(id)}
	}
	h.windows[id] = origin
	return nil
}

// PostMessage records an IPC payload for the window.
func (h *Host) PostMessage(_ context.Context, id domain.WindowID, message []byte) error {
	if err := platform.Require(h, platform.FeatureWebViewMessage); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.windows[id]; !ok {
		return &domain.ErrNotFound{Entity: "window", ID: string(id)}
	}
	cp := append([]byte(nil), message...)
	h.messages = append(h.messages, PostedMessage{Window: id, Message: cp})
	return nil
}

// CloseWindow removes a stub window.
func (h *Host) CloseWindow(_ context.Context, id domain.WindowID) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.windows, id)
	return nil
}

// Messages returns a copy of posted messages (spike diagnostics).
func (h *Host) Messages() []PostedMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]PostedMessage(nil), h.messages...)
}

// DialogOpen always returns explicit unsupported (invariant 14).
func (h *Host) DialogOpen(_ context.Context) error {
	return platform.Require(h, platform.FeatureDialogOpen)
}
