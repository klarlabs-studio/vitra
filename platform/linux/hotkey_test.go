//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestNativeGlobalShortcut(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for X11 global shortcuts")
	}
	if strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != "" {
		t.Skip("Wayland session — global shortcuts stay unsupported")
	}
	runtime.LockOSThread()

	h := New()
	if !h.Features().Available(platform.FeatureGlobalShortcut) {
		t.Fatal("X11 session should advertise FeatureGlobalShortcut")
	}
	if err := h.Open(platform.WindowSpec{ID: "main", Title: "hotkey", Width: 320, Height: 240}, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = h.UnregisterGlobalShortcut("Ctrl+Shift+Y")
		_ = h.CloseWindow(context.Background(), "main")
	})

	if err := h.RegisterGlobalShortcut("Ctrl+Shift+Y", "app.palette"); err != nil {
		t.Fatal(err)
	}
	if err := h.RegisterGlobalShortcut("", "x"); err == nil {
		t.Fatal("expected validation error")
	}
	err := h.RegisterGlobalShortcut("y", "app.palette")
	if err == nil {
		t.Fatal("expected bare key to fail")
	}
	var unsupp *platform.ErrUnsupported
	if errors.As(err, &unsupp) {
		t.Fatalf("unexpected unsupported: %v", err)
	}
	if err := h.UnregisterGlobalShortcut("Ctrl+Shift+Y"); err != nil {
		t.Fatal(err)
	}
	if err := h.UnregisterGlobalShortcut("Ctrl+Shift+Y"); err == nil {
		t.Fatal("expected not-found after unregister")
	}
}
