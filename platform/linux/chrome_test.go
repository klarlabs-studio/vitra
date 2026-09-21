//go:build linux && cgo && vitra_native

package linux

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func TestNativeWindowChrome(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK window chrome")
	}
	runtime.LockOSThread()
	h := New()
	if !h.Features().Available(platform.FeatureWindowChrome) {
		t.Fatal("native host should expose window.chrome")
	}
	if err := h.Open(platform.WindowSpec{ID: "main", Title: "before", Width: 400, Height: 300}, "about:blank", ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = h.CloseWindow(context.Background(), "main")
	})

	want := platform.WindowChrome{
		Title:       "Vitra Chrome",
		Width:       640,
		Height:      480,
		AlwaysOnTop: true,
	}
	if err := h.ApplyWindowChrome("main", want); err != nil {
		t.Fatal(err)
	}
	got, err := h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != want.Title {
		t.Fatalf("title: got %q", got.Title)
	}
	if got.Width != want.Width || got.Height != want.Height {
		t.Fatalf("size: got %dx%d", got.Width, got.Height)
	}
	if !got.AlwaysOnTop {
		t.Fatal("expected always-on-top")
	}
	if got.Maximized || got.Fullscreen {
		t.Fatalf("unexpected presentation: %+v", got)
	}

	maxed := want
	maxed.Maximized = true
	maxed.AlwaysOnTop = false
	if err := h.ApplyWindowChrome("main", maxed); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Maximized || got.Fullscreen || got.AlwaysOnTop {
		t.Fatalf("maximize: %+v", got)
	}

	full := want
	full.Fullscreen = true
	full.AlwaysOnTop = false
	if err := h.ApplyWindowChrome("main", full); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Fullscreen || got.AlwaysOnTop {
		t.Fatalf("fullscreen: %+v", got)
	}

	mini := want
	mini.Minimized = true
	mini.AlwaysOnTop = false
	mini.Fullscreen = false
	if err := h.ApplyWindowChrome("main", mini); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Minimized || got.Hidden {
		t.Fatalf("minimize: %+v", got)
	}

	hidden := want
	hidden.Hidden = true
	hidden.AlwaysOnTop = false
	if err := h.ApplyWindowChrome("main", hidden); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Hidden {
		t.Fatalf("hidden: %+v", got)
	}
	shown := want
	shown.AlwaysOnTop = false
	if err := h.ApplyWindowChrome("main", shown); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hidden || got.Minimized {
		t.Fatalf("shown: %+v", got)
	}

	// 1x1 PNG
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(iconPath, png, 0o644); err != nil {
		t.Fatal(err)
	}
	withIcon := want
	withIcon.AlwaysOnTop = false
	withIcon.IconPath = iconPath
	if err := h.ApplyWindowChrome("main", withIcon); err != nil {
		t.Fatal(err)
	}
	got, err = h.ReadWindowChrome("main")
	if err != nil {
		t.Fatal(err)
	}
	if got.IconPath != iconPath {
		t.Fatalf("icon path: got %q want %q", got.IconPath, iconPath)
	}

	if err := h.ApplyWindowChrome("missing", want); err == nil {
		t.Fatal("expected missing window")
	} else if _, ok := err.(*domain.ErrNotFound); !ok {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := h.ApplyWindowChrome("main", platform.WindowChrome{Title: "x"}); err == nil {
		t.Fatal("expected size validation")
	}
	if err := h.FocusWindow("main"); err != nil {
		t.Fatal(err)
	}
	if err := h.BlurWindow("main"); err != nil {
		t.Fatal(err)
	}
	if err := h.FocusWindow("missing"); err == nil {
		t.Fatal("expected missing window focus")
	} else if _, ok := err.(*domain.ErrNotFound); !ok {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := h.BlurWindow("missing"); err == nil {
		t.Fatal("expected missing window blur")
	} else if _, ok := err.(*domain.ErrNotFound); !ok {
		t.Fatalf("expected not found for blur, got %v", err)
	}
}
