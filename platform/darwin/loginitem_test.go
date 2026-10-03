//go:build darwin && cgo && vitra_native

package darwin

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

// SMAppService registers app bundles only: a bare binary (this test) is told
// so instead of failing obscurely.
func TestLoginItem_UnbundledIsUnsupported(t *testing.T) {
	h := New()
	if f := h.Features()[platform.FeatureLoginItem]; f.Available {
		t.Skipf("running inside an app bundle: %s", f.Detail)
	}
	var unsupp *platform.ErrUnsupported
	if err := h.SetLoginItem("com.example.test", "", true); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureLoginItem {
		t.Fatalf("SetLoginItem unbundled: %v", err)
	}
	if _, err := h.LoginItemEnabled("com.example.test"); !errors.As(err, &unsupp) {
		t.Fatalf("LoginItemEnabled unbundled: %v", err)
	}
}
