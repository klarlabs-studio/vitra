//go:build !windows || !cgo || !vitra_native

package windows

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func TestHost_ExplicitUnsupported(t *testing.T) {
	h := New()
	if h.OS() != platform.OSWindows {
		t.Fatalf("os: %s", h.OS())
	}
	if h.Features().Available(platform.FeatureWindowCreate) {
		t.Fatal("expected window.create unavailable")
	}
	if !h.Features().Available(platform.FeatureOpenURL) {
		t.Fatal("expected browser.open available without native host")
	}
	err := h.Open(platform.WindowSpec{ID: "main"}, "about:blank", "")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowCreate {
		t.Fatalf("got %v", err)
	}
	h.SetInvokeHandler(func(domain.WindowID, domain.Origin, []byte) []byte { return nil })
	h.SetNavPolicy(func(domain.WindowID, string) bool { return false })
	if err := h.ApplyWindowChrome("main", platform.WindowChrome{Width: 100, Height: 100}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowChrome {
		t.Fatalf("chrome: %v", err)
	}
	h.Quit()
}
