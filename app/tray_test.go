package app_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"
	"testing/fstest"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// trayHost is a pluginHost that records the tray it was asked to show.
type trayHost struct {
	*pluginHost
	trays []platform.TraySpec
}

func (h *trayHost) SetTray(spec platform.TraySpec) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.trays = append(h.trays, spec)
	return nil
}

func (h *trayHost) lastTray(t *testing.T) platform.TraySpec {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.trays) == 0 {
		t.Fatal("host tray was never set")
	}
	return h.trays[len(h.trays)-1]
}

func (h *trayHost) trayCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.trays)
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runTrayApp(t *testing.T, assets fstest.MapFS) (*vitra.Runtime, *trayHost, *app.App) {
	t.Helper()
	host := &trayHost{pluginHost: newPluginHost()}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.tray"})
	if err != nil {
		t.Fatal(err)
	}
	assets["index.html"] = &fstest.MapFile{Data: []byte("x")}
	a, err := app.New(app.Options{AppID: "com.example.tray", Assets: assets, Host: host, Runtime: rt, Window: app.WindowOptions{ID: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.UseOfficialPlugins(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() { _ = a.Run(context.Background()) }()
	<-host.ran
	t.Cleanup(a.Quit)
	return rt, host, a
}

// tray.set shows the status in the menu bar: the icon is read from the app's
// assets, never from a path the page chooses on disk.
func TestTraySet_ShowsStatusWithAssetIcon(t *testing.T) {
	icon := pngBytes(t)
	rt, host, _ := runTrayApp(t, fstest.MapFS{"icons/tray.png": {Data: icon}})
	grantAll(t, rt)

	_, err := call(t, rt, "tray.set", map[string]any{
		"tooltip": "Usage", "title": "42%", "icon": "/icons/tray.png", "template": true,
		"items": []any{
			map[string]any{"id": "refresh", "label": "Refresh", "disabled": true},
			map[string]any{"separator": true},
			map[string]any{"id": "pin", "label": "Pin", "checked": true},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	got := host.lastTray(t)
	if got.Tooltip != "Usage" || got.Title != "42%" || !got.Template || !bytes.Equal(got.Icon, icon) {
		t.Fatalf("tray = %+v", got)
	}
	want := []platform.MenuItem{
		{ID: "refresh", Label: "Refresh", Disabled: true},
		{Separator: true},
		{ID: "pin", Label: "Pin", Checked: true},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("items = %+v", got.Items)
	}
	for i := range want {
		if got.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, got.Items[i], want[i])
		}
	}
}

func TestTraySet_RejectsBadIcons(t *testing.T) {
	big := append(pngBytes(t), make([]byte, app.MaxTrayIconBytes)...)
	rt, host, _ := runTrayApp(t, fstest.MapFS{
		"notes.txt": {Data: []byte("not an image")},
		"big.png":   {Data: big},
	})
	grantAll(t, rt)
	for _, icon := range []string{"missing.png", "notes.txt", "big.png", "../index.html"} {
		if _, err := call(t, rt, "tray.set", map[string]any{"icon": icon}, ""); err == nil {
			t.Errorf("tray.set accepted icon %q", icon)
		}
	}
	if n := host.trayCalls(); n != 0 {
		t.Fatalf("host tray set %d times for rejected icons", n)
	}
}

// Without a grant the page cannot touch the tray, whatever the icon.
func TestTraySet_NeedsGrant(t *testing.T) {
	rt, host, _ := runTrayApp(t, fstest.MapFS{"icons/tray.png": {Data: pngBytes(t)}})
	_, err := call(t, rt, "tray.set", map[string]any{"title": "x", "icon": "icons/tray.png"}, "")
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) {
		t.Fatalf("tray.set without grant: %v", err)
	}
	if host.trayCalls() != 0 {
		t.Fatal("host tray set without a grant")
	}
}

// Go code drives the tray directly: it is trusted, so it needs no grant.
func TestAppSetTray_FromGo(t *testing.T) {
	_, host, a := runTrayApp(t, fstest.MapFS{})
	spec := app.TraySpec{Title: "7 GB free", Icon: pngBytes(t), Items: []platform.MenuItem{{ID: "quit", Label: "Quit"}}}
	if err := a.SetTray(spec); err != nil {
		t.Fatal(err)
	}
	if got := host.lastTray(t); got.Title != "7 GB free" || len(got.Items) != 1 {
		t.Fatalf("tray = %+v", got)
	}
	for _, bad := range []app.TraySpec{
		{Icon: []byte("not a png")},
		{Items: []platform.MenuItem{{Label: "no id"}}},
		{Items: []platform.MenuItem{{Separator: true, ID: "x"}}},
	} {
		if err := a.SetTray(bad); err == nil {
			t.Errorf("SetTray accepted %+v", bad)
		}
	}
}

func TestAppSetTray_UnsupportedHost(t *testing.T) {
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.tray"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{AppID: "com.example.tray", Assets: fstest.MapFS{}, Host: host, Runtime: rt, Window: app.WindowOptions{ID: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	var unsupp *platform.ErrUnsupported
	if err := a.SetTray(app.TraySpec{Title: "x"}); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureTray {
		t.Fatalf("SetTray on a host without a tray: %v", err)
	}
}

// Go handlers see native activations too, alongside the page's events.
func TestAppOnAction(t *testing.T) {
	_, host, a := runTrayApp(t, fstest.MapFS{})
	got := make(chan string, 1)
	a.OnAction(func(id string) { got <- id })
	host.action("quit")
	if id := <-got; id != "quit" {
		t.Fatalf("action %q", id)
	}
}
