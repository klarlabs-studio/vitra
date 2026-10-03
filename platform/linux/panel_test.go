//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"os"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// A panel opens hidden, shows centered under the tray anchor, hides itself on
// losing focus, and counts as shown for a moment after that, so the tray
// click that took its focus closes it rather than opening it again.
func TestNativePanel(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK windows")
	}
	h := newHostOnThisThread()
	if !h.Features().Available(platform.FeatureWindowPanel) {
		t.Fatal("native host should expose window.panel")
	}
	if err := h.Open(platform.WindowSpec{ID: "panel", Title: "panel", Width: 300, Height: 200, Kind: platform.WindowKindPanel}, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.CloseWindow(context.Background(), "panel") })
	if err := h.Open(platform.WindowSpec{ID: "main", Title: "main", Width: 300, Height: 200}, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.CloseWindow(context.Background(), "main") })

	if shown, err := h.PanelShown("panel"); err != nil || shown {
		t.Fatalf("panel at open: shown=%v err=%v", shown, err)
	}
	anchor := platform.Rect{X: 400, Y: 0, Width: 30, Height: 24}
	if err := h.ShowPanel("panel", anchor, true); err != nil {
		t.Fatal(err)
	}
	if shown, _ := h.PanelShown("panel"); !shown {
		t.Fatal("panel not shown")
	}
	frame := h.windowFrame("panel")
	if frame.Width != 300 || frame.Height != 200 || frame.X+frame.Width/2 != anchor.X+anchor.Width/2 || frame.Y != anchor.Y+anchor.Height+4 {
		t.Fatalf("panel frame %+v not under anchor %+v", frame, anchor)
	}
	// An icon at the bottom of the screen (a bottom taskbar) opens upward;
	// the panel stays on screen. (Y is below the middle of any test display.)
	bottom := platform.Rect{X: 100, Y: 4000, Width: 24, Height: 24}
	if err := h.ShowPanel("panel", bottom, true); err != nil {
		t.Fatal(err)
	}
	if f := h.windowFrame("panel"); f.Y+f.Height > bottom.Y {
		t.Fatalf("panel frame %+v overlaps a bottom anchor %+v", f, bottom)
	}
	if err := h.HidePanel("panel"); err != nil {
		t.Fatal(err)
	}
	if shown, _ := h.PanelShown("panel"); shown {
		t.Fatal("panel shown after HidePanel")
	}

	_ = h.ShowPanel("panel", anchor, true)
	h.blurPanel("panel")
	if shown, _ := h.PanelShown("panel"); !shown {
		t.Fatal("a click that took the panel's focus should still count as shown")
	}
	time.Sleep(400 * time.Millisecond)
	if shown, _ := h.PanelShown("panel"); shown {
		t.Fatal("panel still counts as shown well after it hid")
	}
	if err := h.ShowPanel("main", anchor, true); err == nil {
		t.Fatal("ShowPanel on a regular window succeeded")
	}
}
