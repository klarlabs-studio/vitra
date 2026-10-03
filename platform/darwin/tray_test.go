//go:build darwin && cgo && vitra_native

package darwin

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

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
