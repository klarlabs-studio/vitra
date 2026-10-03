//go:build darwin && cgo && vitra_native

package darwin

import "go.klarlabs.de/vitra/platform"

// The native host reports the sender URL of every bridge message, and the
// messages WKWebView drops before they reach the app (posted by subframes).
var (
	_ platform.MessageReporter = (*Host)(nil)
	_ platform.RejectReporter  = (*Host)(nil)
)
