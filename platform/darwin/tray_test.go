//go:build darwin && cgo && vitra_native

package darwin

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// The status item shows the title, the icon and the menu item states the
// app set, as AppKit holds them.
func TestTrayStatus(t *testing.T) {
	h := New()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 36, 36))); err != nil {
		t.Fatal(err)
	}
	var state string
	go func() {
		defer h.Quit()
		if err := h.SetTray(platform.TraySpec{
			Tooltip: "Usage", Title: "42%", Icon: buf.Bytes(), Template: true,
			Items: []platform.MenuItem{
				{ID: "refresh", Label: "Refresh", Disabled: true},
				{Separator: true},
				{ID: "pin", Label: "Pin", Checked: true},
				{ID: "quit", Label: "Quit"},
			},
		}); err != nil {
			t.Error(err)
			return
		}
		state = h.trayState()
		// Without icon or title the item falls back to the app name.
		if err := h.SetTray(platform.TraySpec{}); err != nil {
			t.Error(err)
			return
		}
		if title := strings.SplitN(h.trayState(), "\n", 2)[0]; title == "" {
			t.Error("empty status item: no title and no icon")
		}
		h.ClearTray()
	}()
	onMainThread(func() { _ = h.Run() })

	want := "42%\n1\n1\nRefresh\t2\n-\nPin\t4\nQuit\t0\n"
	if state != want {
		t.Fatalf("tray state:\n%q\nwant\n%q", state, want)
	}
}

// With ClickActivates a left click is reported with the status item's
// rectangle, and the menu no longer opens on it.
func TestTrayClickActivates(t *testing.T) {
	h := New()
	clicks := make(chan platform.TrayClick, 1)
	h.SetTrayClickHandler(func(c platform.TrayClick) { clicks <- c })
	var anchor platform.Rect
	var anchorErr error
	var state string
	go func() {
		defer h.Quit()
		if err := h.SetTray(platform.TraySpec{
			Title: "42%", ClickActivates: true,
			Items: []platform.MenuItem{{ID: "quit", Label: "Quit"}},
		}); err != nil {
			t.Error(err)
			return
		}
		state = h.trayState()
		// The menu bar places the item on a later layout pass; wait until
		// it stops moving.
		var last platform.Rect
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			anchor, anchorErr = h.TrayAnchor()
			if anchorErr == nil && anchor == last {
				break
			}
			last = anchor
		}
		h.clickTray()
		h.ClearTray()
	}()
	onMainThread(func() { _ = h.Run() })

	if anchorErr != nil || anchor.Width <= 0 || anchor.Height <= 0 {
		t.Fatalf("TrayAnchor = %+v, %v", anchor, anchorErr)
	}
	select {
	case c := <-clicks:
		if !c.HasAnchor || c.Anchor != anchor {
			t.Fatalf("click %+v, want anchor %+v", c, anchor)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no tray click reported")
	}
	// The menu is kept for right clicks; the item lists it.
	if !strings.Contains(state, "Quit\t0") {
		t.Fatalf("tray state %q", state)
	}
}
