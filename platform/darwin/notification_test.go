//go:build darwin && cgo && vitra_native

package darwin

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

// macOS delivers notifications only for apps with a bundle identifier. An
// unbundled binary (go run, go test) must say so instead of silently showing
// nothing.
func TestNotification_UnbundledIsExplicitlyUnsupported(t *testing.T) {
	h := New()
	if h.Features().Available(platform.FeatureNotificationShow) {
		t.Fatal("an unbundled test binary must not claim notifications")
	}
	err := h.ShowNotification("title", "body")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureNotificationShow || unsupp.Detail == "" {
		t.Fatalf("ShowNotification = %v, want ErrUnsupported with a detail", err)
	}
}
