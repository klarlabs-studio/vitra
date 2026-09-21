//go:build !linux || !cgo || !vitra_native

package linux

import (
	"context"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func TestStubHost_ReportsNativeRequirement(t *testing.T) {
	h := New()
	if h.OS() != platform.OSLinux {
		t.Fatalf("os: %s", h.OS())
	}
	fs := h.Features()
	for _, f := range []platform.Feature{
		platform.FeatureWindowCreate,
		platform.FeatureClipboard,
		platform.FeatureDialogOpen,
		platform.FeatureMenuBar,
		platform.FeatureTray,
	} {
		if fs.Available(f) {
			t.Fatalf("stub should not claim %s", f)
		}
	}

	h.SetInvokeHandler(func(domain.WindowID, domain.Origin, []byte) []byte { return nil })
	h.SetNavPolicy(func(domain.WindowID, string) bool { return false })
	h.SetActionHandler(func(string) {})

	mustErr := func(err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected error")
		}
	}
	mustErr(h.CreateWindow(context.Background(), platform.WindowSpec{ID: "main"}))
	mustErr(h.Open(platform.WindowSpec{ID: "main"}, "about:blank", ""))
	mustErr(h.NavigateWindow(context.Background(), "main", domain.OriginPackagedLocal))
	mustErr(h.PostMessage(context.Background(), "main", []byte("{}")))
	mustErr(h.Eval("main", "1"))
	mustErr(h.CloseWindow(context.Background(), "main"))
	_, err := h.ClipboardGet()
	mustErr(err)
	mustErr(h.ClipboardSet("x"))
	_, err = h.OpenFileDialog()
	mustErr(err)
	_, err = h.SaveFileDialog()
	mustErr(err)
	_, err = h.OpenDirectoryDialog()
	mustErr(err)
	mustErr(h.SetMenuBar("main", []platform.MenuItem{{Menu: "File", ID: "quit", Label: "Quit"}}))
	mustErr(h.SetTray("tip", []platform.MenuItem{{ID: "quit", Label: "Quit"}}))
	h.ClearTray()
	if held, release, err := h.TrySingleInstance("vitra-stub-test"); err != nil || !held {
		t.Fatalf("single-instance: held=%v err=%v", held, err)
	} else {
		release()
	}
	if !fs.Available(platform.FeatureSingleInstance) {
		t.Fatal("stub should expose single-instance via flock")
	}
	if fs.Available(platform.FeatureDialogSave) {
		t.Fatal("stub should not claim dialog.save without native host")
	}
	if fs.Available(platform.FeatureDragDrop) {
		t.Fatal("stub should not claim drag_drop without native host")
	}
	h.SetDragDropHandler(func(domain.WindowID, []string) {})
	mustErr(h.EnableDragDrop("main", true))
	if fs.Available(platform.FeatureWindowChrome) {
		t.Fatal("stub should not claim window.chrome without native host")
	}
	mustErr(h.ApplyWindowChrome("main", platform.WindowChrome{Title: "x", Width: 100, Height: 100}))
	if _, err := h.ReadWindowChrome("main"); err == nil {
		t.Fatal("expected read chrome error")
	}
	if !fs.Available(platform.FeatureOpenURL) {
		t.Fatal("stub should expose browser.open via xdg-open")
	}
	h.Quit()
	mustErr(h.Run())
}
