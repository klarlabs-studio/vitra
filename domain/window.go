package domain

import (
	"errors"
	"sync"
	"time"
)

// WindowState is the lifecycle state of a window / WebView.
type WindowState string

const (
	WindowStateOpen   WindowState = "open"
	WindowStateClosed WindowState = "closed"
)

// Window is the aggregate root for a desktop window hosting a WebView.
// A newly created window has no privileged native API access; authority
// comes only from capability grants that match its identity and origin.
type Window struct {
	mu     sync.RWMutex
	id     WindowID
	origin Origin
	state  WindowState
	opened time.Time
	closed time.Time
}

// NewWindow creates an open window at the given origin.
func NewWindow(id WindowID, origin Origin) (*Window, error) {
	if id == "" {
		return nil, errors.New("window id is required")
	}
	if origin == "" {
		return nil, errors.New("window origin is required")
	}
	return &Window{
		id:     id,
		origin: origin,
		state:  WindowStateOpen,
		opened: time.Now().UTC(),
	}, nil
}

// ID returns the window identity.
func (w *Window) ID() WindowID {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.id
}

// Origin returns the current content origin.
func (w *Window) Origin() Origin {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.origin
}

// State returns the lifecycle state.
func (w *Window) State() WindowState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// IsOpen reports whether the window can still initiate IPC.
func (w *Window) IsOpen() bool {
	return w.State() == WindowStateOpen
}

// Navigate changes the window's content origin.
// Capability authority does not follow navigation: grants are evaluated
// against the current origin, so a grant for app://local does not apply
// after navigating to an untrusted remote origin.
func (w *Window) Navigate(next Origin) error {
	if next == "" {
		return &ErrValidation{Message: "navigation origin must not be empty"}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state != WindowStateOpen {
		return &ErrValidation{Message: "cannot navigate a closed window"}
	}
	w.origin = next
	return nil
}

// Close marks the window closed. Associated subscriptions and resource
// ownership should be released by the runtime after Close.
func (w *Window) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state == WindowStateClosed {
		return nil
	}
	w.state = WindowStateClosed
	w.closed = time.Now().UTC()
	return nil
}

// Caller returns the current IPC caller identity for this window.
func (w *Window) Caller() (Caller, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.state != WindowStateOpen {
		return Caller{}, &ErrDenied{
			Window: w.id,
			Origin: w.origin,
			Code:   DenialWindowClosed,
			Reason: "window is closed",
		}
	}
	return NewCaller(w.id, w.origin)
}
