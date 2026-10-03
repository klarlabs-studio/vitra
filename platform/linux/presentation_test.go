//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"os"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// An accessory app keeps its windows, open and later, out of taskbars.
func TestNativePresentation(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK windows")
	}
	h := newHostOnThisThread()
	if !h.Features().Available(platform.FeaturePresentation) {
		t.Fatal("native host should expose app.presentation")
	}
	open := func(id string) {
		t.Helper()
		if err := h.Open(platform.WindowSpec{ID: domain.WindowID(id), Title: id, Width: 200, Height: 100}, "about:blank", ""); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = h.CloseWindow(context.Background(), domain.WindowID(id)) })
	}
	open("before")
	if h.skipsTaskbar("before") {
		t.Fatal("regular window skips the taskbar")
	}
	if err := h.SetPresentation(platform.PresentationAccessory); err != nil {
		t.Fatal(err)
	}
	open("after")
	if !h.skipsTaskbar("before") || !h.skipsTaskbar("after") {
		t.Fatal("accessory windows show in the taskbar")
	}
	if err := h.SetPresentation(platform.PresentationRegular); err != nil {
		t.Fatal(err)
	}
	if h.skipsTaskbar("before") {
		t.Fatal("regular presentation kept the window out of the taskbar")
	}
	if err := h.SetPresentation("floating"); err == nil {
		t.Fatal("unknown presentation accepted")
	}
}
