package official_test

import (
	"context"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official/browser"
	"go.klarlabs.de/vitra/plugin/official/clipboard"
	"go.klarlabs.de/vitra/plugin/official/dialog"
	"go.klarlabs.de/vitra/plugin/official/dragdrop"
	"go.klarlabs.de/vitra/plugin/official/fs"
	"go.klarlabs.de/vitra/plugin/official/menu"
	"go.klarlabs.de/vitra/plugin/official/notification"
	officialos "go.klarlabs.de/vitra/plugin/official/os"
	officialpath "go.klarlabs.de/vitra/plugin/official/path"
	"go.klarlabs.de/vitra/plugin/official/tray"
	officialwindow "go.klarlabs.de/vitra/plugin/official/window"
)

func TestOfficialPlugins_RegisterCleanly(t *testing.T) {
	reg := plugin.NewRegistry(plugin.SemVer{Major: 0, Minor: 3, Patch: 0})
	for _, p := range []plugin.Plugin{
		fs.New(), dialog.New(), clipboard.New(), browser.New(),
		officialos.New(), notification.New(), officialpath.New(), officialwindow.New(), menu.New(), tray.New(), dragdrop.New(),
	} {
		if err := reg.Register(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []domain.PluginID{
		fs.PluginID, clipboard.PluginID, browser.PluginID,
		officialos.PluginID, notification.PluginID, officialpath.PluginID, officialwindow.PluginID, menu.PluginID, tray.PluginID, dragdrop.PluginID,
	} {
		if _, err := reg.Get(id); err != nil {
			t.Fatal(err)
		}
	}
	surface := reg.InspectSurface()
	if len(surface) < 12 {
		t.Fatalf("expected official plugin permissions, got %v", surface)
	}
}
