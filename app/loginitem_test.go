package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"

	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/plugin/official"
)

// loginHost is a tray host that keeps login items in memory.
type loginHost struct {
	*trayHost
	lmu   sync.Mutex
	items map[string]string // app id -> exec path
}

func (h *loginHost) Features() platform.FeatureSet {
	fs := h.trayHost.Features()
	fs[platform.FeatureLoginItem] = platform.Support{Feature: platform.FeatureLoginItem, Available: true}
	return fs
}

func (h *loginHost) LoginItemEnabled(appID string) (bool, error) {
	h.lmu.Lock()
	defer h.lmu.Unlock()
	_, ok := h.items[appID]
	return ok, nil
}

func (h *loginHost) SetLoginItem(appID, execPath string, enabled bool) error {
	h.lmu.Lock()
	defer h.lmu.Unlock()
	if enabled {
		h.items[appID] = execPath
	} else {
		delete(h.items, appID)
	}
	return nil
}

// The page toggles launch at login through app.setLoginItem, which needs the
// app.login_item grant, and the host registers this executable.
func TestLoginItem_OfficialCommands(t *testing.T) {
	host := &loginHost{trayHost: &trayHost{pluginHost: newPluginHost()}, items: map[string]string{}}
	rt, a := runPluginAppOn(t, host, host.ran)

	if _, err := call(t, rt, "app.setLoginItem", map[string]any{"enabled": true}, ""); err == nil {
		t.Fatal("app.setLoginItem without a grant succeeded")
	}
	grantAll(t, rt)
	if out, err := call(t, rt, "app.loginItem", nil, ""); err != nil || out != (official.LoginItem{Enabled: false}) {
		t.Fatalf("app.loginItem = %#v, %v", out, err)
	}
	if _, err := call(t, rt, "app.setLoginItem", map[string]any{"enabled": true}, ""); err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	if got := host.items["com.example.plugins"]; got != exe {
		t.Fatalf("registered %q, want the running executable %q", got, exe)
	}
	if on, err := a.LoginItemEnabled(); err != nil || !on {
		t.Fatalf("LoginItemEnabled = %v, %v", on, err)
	}
	if _, err := call(t, rt, "app.setLoginItem", map[string]any{"enabled": false}, ""); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.LoginItemEnabled(); on {
		t.Fatal("still enabled after app.setLoginItem false")
	}
	if _, err := call(t, rt, "app.setLoginItem", map[string]any{}, ""); err == nil {
		t.Fatal("app.setLoginItem without enabled succeeded")
	}
}

func TestLoginItem_UnsupportedHost(t *testing.T) {
	_, _, a := runTrayApp(t, fstest.MapFS{})
	var unsupp *platform.ErrUnsupported
	if err := a.SetLoginItem(true); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureLoginItem {
		t.Fatalf("SetLoginItem on a host without login items: %v", err)
	}
	if _, err := a.LoginItemEnabled(); !errors.As(err, &unsupp) {
		t.Fatalf("LoginItemEnabled on a host without login items: %v", err)
	}
}
