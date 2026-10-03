//go:build darwin && cgo && vitra_native

package darwin

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// A panel opens hidden, shows centered under the tray anchor, hides itself
// on losing focus, and counts as shown for a moment after that, so the tray
// click that took its focus closes it rather than opening it again.
func TestPanel(t *testing.T) {
	h := New()
	var (
		openedHidden, shownAfterShow, shownAfterHide bool
		guard, afterGuard                            bool
		frame                                        platform.Rect
		notPanel                                     error
	)
	anchor := platform.Rect{X: 400, Y: 0, Width: 30, Height: 24}
	go func() {
		defer h.Quit()
		if err := h.Open(platform.WindowSpec{ID: "panel", Title: "Panel", Width: 300, Height: 200, Kind: platform.WindowKindPanel}, "about:blank", ""); err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = h.CloseWindow(context.Background(), "panel") }()
		if err := h.Open(platform.WindowSpec{ID: "main", Title: "Main", Width: 300, Height: 200}, "about:blank", ""); err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = h.CloseWindow(context.Background(), "main") }()
		shown, _ := h.PanelShown("panel")
		openedHidden = !shown
		if err := h.ShowPanel("panel", anchor, true); err != nil {
			t.Error(err)
			return
		}
		shownAfterShow, _ = h.PanelShown("panel")
		frame = h.windowFrame("panel")
		if err := h.HidePanel("panel"); err != nil {
			t.Error(err)
			return
		}
		shownAfterHide, _ = h.PanelShown("panel")
		_ = h.ShowPanel("panel", anchor, true)
		h.blurPanel("panel")
		guard, _ = h.PanelShown("panel")
		time.Sleep(400 * time.Millisecond)
		afterGuard, _ = h.PanelShown("panel")
		notPanel = h.ShowPanel("main", anchor, true)
	}()
	onMainThread(func() { _ = h.Run() })

	if !openedHidden || !shownAfterShow || shownAfterHide {
		t.Fatalf("openedHidden=%v shownAfterShow=%v shownAfterHide=%v", openedHidden, shownAfterShow, shownAfterHide)
	}
	if frame.Width != 300 || frame.Height != 200 {
		t.Fatalf("panel frame %+v", frame)
	}
	// Centered under the anchor, below it (unless clamped by the menu bar).
	if mid := frame.X + frame.Width/2; mid != anchor.X+anchor.Width/2 || frame.Y < anchor.Y+anchor.Height {
		t.Fatalf("panel frame %+v not under anchor %+v", frame, anchor)
	}
	if !guard || afterGuard {
		t.Fatalf("after losing focus: shown=%v, a moment later shown=%v", guard, afterGuard)
	}
	if notPanel == nil {
		t.Fatal("ShowPanel on a regular window succeeded")
	}
}
