//go:build !linux || !cgo || !vitra_native

package linux

import (
	"context"
	"errors"
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
	_, err = h.OpenFileDialog(platform.DialogFileOptions{})
	mustErr(err)
	_, err = h.OpenFilesDialog(platform.DialogFileOptions{Multiple: true})
	mustErr(err)
	_, err = h.SaveFileDialog(platform.DialogFileOptions{})
	mustErr(err)
	_, err = h.OpenDirectoryDialog(platform.DialogFileOptions{})
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
	mustErr(h.FocusWindow("main"))
	mustErr(h.BlurWindow("main"))
	if !fs.Available(platform.FeatureOpenURL) {
		t.Fatal("stub should expose browser.open via xdg-open")
	}
	h.Quit()
	mustErr(h.Run())
}

// The stub fails like the macOS and Windows stubs: an explicit
// ErrUnsupported naming the feature, so callers can tell what is missing.
func TestStubHost_ReturnsErrUnsupportedPerFeature(t *testing.T) {
	h := New()
	ctx := context.Background()
	second := func(_ any, err error) error { return err }
	cases := []struct {
		name    string
		err     error
		feature platform.Feature
	}{
		{"CreateWindow", h.CreateWindow(ctx, platform.WindowSpec{ID: "main"}), platform.FeatureWindowCreate},
		{"Open", h.Open(platform.WindowSpec{ID: "main"}, "about:blank", ""), platform.FeatureWindowCreate},
		{"CloseWindow", h.CloseWindow(ctx, "main"), platform.FeatureWindowCreate},
		{"Run", h.Run(), platform.FeatureWindowCreate},
		{"NavigateWindow", h.NavigateWindow(ctx, "main", domain.OriginPackagedLocal), platform.FeatureWindowNavigate},
		{"PostMessage", h.PostMessage(ctx, "main", []byte("{}")), platform.FeatureWebViewMessage},
		{"Eval", h.Eval("main", "1"), platform.FeatureWebViewMessage},
		{"ClipboardGet", second(h.ClipboardGet()), platform.FeatureClipboard},
		{"ClipboardSet", h.ClipboardSet("x"), platform.FeatureClipboard},
		{"OpenFileDialog", second(h.OpenFileDialog(platform.DialogFileOptions{})), platform.FeatureDialogOpen},
		{"OpenFilesDialog", second(h.OpenFilesDialog(platform.DialogFileOptions{Multiple: true})), platform.FeatureDialogOpen},
		{"SaveFileDialog", second(h.SaveFileDialog(platform.DialogFileOptions{})), platform.FeatureDialogSave},
		{"OpenDirectoryDialog", second(h.OpenDirectoryDialog(platform.DialogFileOptions{})), platform.FeatureDialogOpenDirectory},
		{"MessageDialog", second(h.MessageDialog("info", "t", "m")), platform.FeatureDialogMessage},
		{"ShowNotification", h.ShowNotification("t", "b"), platform.FeatureNotificationShow},
		{"SetMenuBar", h.SetMenuBar("main", nil), platform.FeatureMenuBar},
		{"ActivateMenuAccel", second(h.ActivateMenuAccel("main", "Ctrl+Q")), platform.FeatureMenuBar},
		{"SetTray", h.SetTray("tip", nil), platform.FeatureTray},
		{"RegisterGlobalShortcut", h.RegisterGlobalShortcut("id", "Ctrl+K"), platform.FeatureGlobalShortcut},
		{"UnregisterGlobalShortcut", h.UnregisterGlobalShortcut("id"), platform.FeatureGlobalShortcut},
		{"EnableDragDrop", h.EnableDragDrop("main", true), platform.FeatureDragDrop},
		{"ApplyWindowChrome", h.ApplyWindowChrome("main", platform.WindowChrome{}), platform.FeatureWindowChrome},
		{"ReadWindowChrome", second(h.ReadWindowChrome("main")), platform.FeatureWindowChrome},
		{"FocusWindow", h.FocusWindow("main"), platform.FeatureWindowChrome},
		{"BlurWindow", h.BlurWindow("main"), platform.FeatureWindowChrome},
	}
	for _, c := range cases {
		var unsupp *platform.ErrUnsupported
		if !errors.As(c.err, &unsupp) {
			t.Errorf("%s: want *platform.ErrUnsupported, got %v", c.name, c.err)
			continue
		}
		if unsupp.Feature != c.feature || unsupp.OS != platform.OSLinux {
			t.Errorf("%s: got feature %s on %s, want %s on linux", c.name, unsupp.Feature, unsupp.OS, c.feature)
		}
	}
}
