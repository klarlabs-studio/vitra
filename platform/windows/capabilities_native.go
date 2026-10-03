//go:build windows && cgo && vitra_native

package windows

import "go.klarlabs.de/vitra/platform"

// The native host reports the sender URL of every bridge message.
var _ platform.MessageReporter = (*Host)(nil)
