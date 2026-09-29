//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

func TestNativeMenuAccelerator(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK menu accelerators")
	}
	runtime.LockOSThread()

	h := New()
	if os.Getenv("WAYLAND_DISPLAY") != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") {
		if h.Features().Available(platform.FeatureGlobalShortcut) {
			t.Fatal("Linux must not claim global shortcuts on Wayland")
		}
	} else if os.Getenv("DISPLAY") != "" && !h.Features().Available(platform.FeatureGlobalShortcut) {
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
}
