//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"os"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// The host keeps delivering native callbacks after its last window closes:
// a menu bar app lives on in its tray and opens windows again later.
func TestNativeHostOutlivesLastWindow(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK windows")
	}
	h := newHostOnThisThread()
	got := make(chan string, 1)
	h.SetActionHandler(func(id string) { got <- id })
	spec := platform.WindowSpec{ID: "main", Title: "main", Width: 300, Height: 200}
	if err := h.Open(spec, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	if err := h.CloseWindow(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	if err := h.Open(spec, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.CloseWindow(context.Background(), "main") })
	if err := h.SetMenuBar("main", []platform.MenuItem{{Menu: "File", ID: "again", Label: "Again", Shortcut: "Ctrl+G"}}); err != nil {
		t.Fatal(err)
	}
	if ok, err := h.ActivateMenuAccel("main", "Ctrl+G"); err != nil || !ok {
		t.Fatalf("accelerator: %v %v", ok, err)
	}
	select {
	case id := <-got:
		if id != "again" {
			t.Fatalf("action %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no action after the first window closed")
	}
}
