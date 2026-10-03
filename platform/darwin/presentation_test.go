//go:build darwin && cgo && vitra_native

package darwin

import (
	"testing"

	"go.klarlabs.de/vitra/platform"
)

// An accessory app runs without a Dock icon; it can switch back.
func TestSetPresentation(t *testing.T) {
	h := New()
	var accessory, regular bool
	go func() {
		defer h.Quit()
		if err := h.SetPresentation(platform.PresentationAccessory); err != nil {
			t.Error(err)
			return
		}
		accessory = h.isAccessory()
		if err := h.SetPresentation(platform.PresentationRegular); err != nil {
			t.Error(err)
			return
		}
		regular = !h.isAccessory()
		if err := h.SetPresentation("floating"); err == nil {
			t.Error("unknown presentation accepted")
		}
	}()
	onMainThread(func() { _ = h.Run() })
	if !accessory || !regular {
		t.Fatalf("accessory=%v regular=%v", accessory, regular)
	}
}
