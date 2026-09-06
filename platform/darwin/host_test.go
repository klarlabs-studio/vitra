package darwin

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func TestHost_ExplicitUnsupported(t *testing.T) {
	h := New()
	if h.OS() != platform.OSDarwin {
		t.Fatalf("os: %s", h.OS())
	}
	if h.Features().Available(platform.FeatureWindowCreate) {
		t.Fatal("expected window.create unavailable")
	}
	err := h.Open(platform.WindowSpec{ID: "main"}, "about:blank", "")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowCreate {
		t.Fatalf("got %v", err)
	}
	h.SetInvokeHandler(func(domain.WindowID, domain.Origin, []byte) []byte { return nil })
	h.SetNavPolicy(func(domain.WindowID, string) bool { return false })
	h.Quit()
}
