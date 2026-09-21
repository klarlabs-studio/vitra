//go:build !windows || !cgo || !vitra_native

package windows

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func TestHost_ExplicitUnsupported(t *testing.T) {
	h := New()
	if h.OS() != platform.OSWindows {
		t.Fatalf("os: %s", h.OS())
	}
	if h.Features().Available(platform.FeatureWindowCreate) {
		t.Fatal("expected window.create unavailable")
	}
	if !h.Features().Available(platform.FeatureOpenURL) {
		t.Fatal("expected browser.open available without native host")
	}
	err := h.Open(platform.WindowSpec{ID: "main"}, "about:blank", "")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowCreate {
		t.Fatalf("got %v", err)
	}
	h.SetInvokeHandler(func(domain.WindowID, domain.Origin, []byte) []byte { return nil })
	h.SetNavPolicy(func(domain.WindowID, string) bool { return false })
	if _, err := h.OpenFileDialog(platform.DialogFileOptions{}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDialogOpen {
		t.Fatalf("open dialog: %v", err)
	}
	if _, err := h.SaveFileDialog(platform.DialogFileOptions{}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDialogSave {
		t.Fatalf("save dialog: %v", err)
	}
	if _, err := h.OpenDirectoryDialog(); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDialogOpenDirectory {
		t.Fatalf("open directory dialog: %v", err)
	}
	if err := h.ApplyWindowChrome("main", platform.WindowChrome{Width: 100, Height: 100}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowChrome {
		t.Fatalf("chrome: %v", err)
	}
	if err := h.SetMenuBar("main", []platform.MenuItem{{Menu: "File", ID: "quit", Label: "Quit"}}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureMenuBar {
		t.Fatalf("menu: %v", err)
	}
	if err := h.SetTray("tip", nil); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureTray {
		t.Fatalf("tray: %v", err)
	}
	if err := h.EnableDragDrop("main", true); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDragDrop {
		t.Fatalf("dragdrop: %v", err)
	}
	if err := h.RegisterGlobalShortcut("Ctrl+Shift+P", "app.palette"); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureGlobalShortcut {
		t.Fatalf("shortcut: %v", err)
	}
	h.Quit()
}
