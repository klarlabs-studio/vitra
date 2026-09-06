//go:build !linux || !cgo || !vitra_native

package linux

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestStubHost_ReportsNativeRequirement(t *testing.T) {
	h := New()
	if h.OS() != platform.OSLinux {
		t.Fatalf("os: %s", h.OS())
	}
	if h.Features().Available(platform.FeatureWindowCreate) {
		t.Fatal("stub should not claim window.create")
	}
	err := h.Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, err) && err.Error() == "" {
		t.Fatal("empty error")
	}
}
