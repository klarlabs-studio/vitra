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
	mustErr(h.SetMenuBar("main", []MenuItem{{Menu: "File", ID: "quit", Label: "Quit"}}))
	mustErr(h.SetTray("tip"))
	h.ClearTray()
	h.Quit()
	mustErr(h.Run())
}
