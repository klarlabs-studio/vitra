package domain

import "errors"

// Caller is the validated identity of an IPC sender at the native boundary.
// Frontend-provided identity is never trusted; adapters must populate Caller
// from the WebView / window host, not from message payload fields.
type Caller struct {
	Window WindowID
	Origin Origin
}

// NewCaller constructs a Caller from a window id and origin.
func NewCaller(window WindowID, origin Origin) (Caller, error) {
	if window == "" {
		return Caller{}, errors.New("caller window must not be empty")
	}
	if origin == "" {
		return Caller{}, errors.New("caller origin must not be empty")
	}
	return Caller{Window: window, Origin: origin}, nil
}
