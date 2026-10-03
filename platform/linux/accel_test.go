//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

func TestNativeMenuAccelerator(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK menu accelerators")
	}

	h := newHostOnThisThread()
	// On Wayland the claim follows the GlobalShortcuts portal (portal_test.go).
	wayland := os.Getenv("WAYLAND_DISPLAY") != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
	if !wayland && !h.Features().Available(platform.FeatureGlobalShortcut) {
		t.Fatal("X11 session should claim FeatureGlobalShortcut")
	}
	// Action handlers run off the UI thread, so wait for the delivery.
	got := make(chan string, 4)
	h.SetActionHandler(func(id string) { got <- id })
	expect := func(want string) {
		t.Helper()
		select {
		case id := <-got:
			if id != want {
				t.Fatalf("action id=%q, want %q", id, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no action delivered, want %q", want)
		}
	}
	if err := h.Open(platform.WindowSpec{ID: "main", Title: "accel", Width: 400, Height: 300}, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = h.CloseWindow(context.Background(), "main")
	})

	if err := h.SetMenuBar("main", []platform.MenuItem{
		{Menu: "File", ID: "app.quit", Label: "Quit", Shortcut: "Ctrl+Q"},
	}); err != nil {
		t.Fatal(err)
	}
	ok, err := h.ActivateMenuAccel("main", "Ctrl+Q")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected accelerator to activate")
	}
	expect("app.quit")

	// Clearing the menu must drop the prior accelerator.
	if err := h.SetMenuBar("main", nil); err != nil {
		t.Fatal(err)
	}
	ok, err = h.ActivateMenuAccel("main", "Ctrl+Q")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected cleared accelerator to miss")
	}
	select {
	case id := <-got:
		t.Fatalf("unexpected action after clear: %q", id)
	case <-time.After(200 * time.Millisecond):
	}

	// GTK-style form should also work.
	if err := h.SetMenuBar("main", []platform.MenuItem{
		{Menu: "File", ID: "help.about", Label: "About", Shortcut: "<Control>a"},
	}); err != nil {
		t.Fatal(err)
	}
	ok, err = h.ActivateMenuAccel("main", "<Control>a")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected GTK-form accelerator")
	}
	expect("help.about")

	// Disabled items never fire; a checked item fires and stays checked.
	if err := h.SetMenuBar("main", []platform.MenuItem{
		{Menu: "File", ID: "file.save", Label: "Save", Shortcut: "Ctrl+S", Disabled: true},
		{Menu: "File", Separator: true},
		{Menu: "File", ID: "view.pin", Label: "Pin", Shortcut: "Ctrl+P", Checked: true},
	}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := h.ActivateMenuAccel("main", "Ctrl+S"); ok {
		t.Fatal("disabled item's accelerator fired")
	}
	if ok, _ := h.ActivateMenuAccel("main", "Ctrl+P"); !ok {
		t.Fatal("expected checked item's accelerator")
	}
	expect("view.pin")
}
