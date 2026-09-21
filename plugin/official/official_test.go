package official_test

import (
	"context"
	"testing"

	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official/browser"
	"go.klarlabs.de/vitra/plugin/official/clipboard"
	"go.klarlabs.de/vitra/plugin/official/dialog"
	"go.klarlabs.de/vitra/plugin/official/fs"
	officialos "go.klarlabs.de/vitra/plugin/official/os"
)

func TestOfficialPlugins_RegisterCleanly(t *testing.T) {
	reg := plugin.NewRegistry(plugin.SemVer{Major: 0, Minor: 3, Patch: 0})
	if err := reg.Register(context.Background(), fs.New()); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(context.Background(), dialog.New()); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(context.Background(), clipboard.New()); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(context.Background(), browser.New()); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(context.Background(), officialos.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get(fs.PluginID); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get(clipboard.PluginID); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get(browser.PluginID); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get(officialos.PluginID); err != nil {
		t.Fatal(err)
	}
	surface := reg.InspectSurface()
	if len(surface) < 8 {
		t.Fatalf("expected fs+dialog+clipboard+browser+os permissions, got %v", surface)
	}
}
