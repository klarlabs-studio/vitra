//go:build linux && cgo && vitra_native

package linux

import "go.klarlabs.de/vitra/platform"

// The native host reports the sender URL of every bridge message.
var _ platform.MessageReporter = (*Host)(nil)
