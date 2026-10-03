package app_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// panelHost is an accessory tray host with panels. It records what the app
// asked of each panel.
type panelHost struct {
	*accessoryHost
	click  func(platform.TrayClick)
	anchor platform.Rect
	pmx    sync.Mutex
	shown  map[domain.WindowID]bool
	shows  []platform.Rect
	hides  int
}

func newPanelHost() *panelHost {
	return &panelHost{accessoryHost: newAccessoryHost(), shown: map[domain.WindowID]bool{}, anchor: platform.Rect{X: 900, Width: 24, Height: 22}}
}

func (h *panelHost) Features() platform.FeatureSet {
	fs := h.accessoryHost.Features()
	for _, f := range []platform.Feature{platform.FeatureWindowPanel, platform.FeatureTrayAnchor} {
		fs[f] = platform.Support{Feature: f, Available: true}
	}
	return fs
}

func (h *panelHost) SetTrayClickHandler(fn func(platform.TrayClick)) { h.click = fn }
func (h *panelHost) TrayAnchor() (platform.Rect, error)              { return h.anchor, nil }

func (h *panelHost) ShowPanel(id domain.WindowID, anchor platform.Rect, has bool) error {
	h.pmx.Lock()
	defer h.pmx.Unlock()
	if !has {
		anchor = platform.Rect{X: -1}
	}
	h.shown[id] = true
	h.shows = append(h.shows, anchor)
	return nil
}

func (h *panelHost) HidePanel(id domain.WindowID) error {
	h.pmx.Lock()
	defer h.pmx.Unlock()
	h.shown[id] = false
	h.hides++
	return nil
}

func (h *panelHost) PanelShown(id domain.WindowID) (bool, error) {
	h.pmx.Lock()
	defer h.pmx.Unlock()
	return h.shown[id], nil
}

func (h *panelHost) counts() (shows []platform.Rect, hides int) {
	h.pmx.Lock()
	defer h.pmx.Unlock()
	return append([]platform.Rect(nil), h.shows...), h.hides
}

func newPanelApp(t *testing.T, host *panelHost) (*vitra.Runtime, *app.App) {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.bar"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID: "com.example.bar", Assets: fstest.MapFS{"index.html": {Data: []byte("x")}},
		Host: host, Runtime: rt, Presentation: app.PresentationAccessory,
		Window: app.WindowOptions{ID: "panel", Kind: app.WindowKindPanel, Width: 360, Height: 480},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rt, a
}

// The menu bar app's primary window is a panel: Run opens it hidden as a
// panel, and a tray click shows it under the icon, then hides it.
func TestTrayPanel_ToggledByTrayClick(t *testing.T) {
	host := newPanelHost()
	rt, a := newPanelApp(t, host)
	if err := a.UseOfficialPlugins(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTray(app.TraySpec{Title: "42%", Panel: "panel"}); err != nil {
		t.Fatal(err)
	}
	runAsync(t, a, host.ran)
	if len(host.opens) != 1 || host.opens[0].Kind != platform.WindowKindPanel {
		t.Fatalf("opened %+v", host.opens)
	}
	if !host.lastTray(t).ClickActivates {
		t.Fatal("a tray panel needs ClickActivates")
	}
	if _, err := rt.SubscribeEvent("panel-click", "tray.click", "panel"); err != nil {
		t.Fatal(err)
	}

	clickAnchor := platform.Rect{X: 880, Y: 0, Width: 30, Height: 24}
	host.click(platform.TrayClick{Anchor: clickAnchor, HasAnchor: true})
	waitFor(t, func() bool { s, _ := host.counts(); return len(s) == 1 })
	if shows, _ := host.counts(); shows[0] != clickAnchor {
		t.Fatalf("panel shown at %+v, want the click's anchor", shows[0])
	}
	host.click(platform.TrayClick{Anchor: clickAnchor, HasAnchor: true})
	waitFor(t, func() bool { _, h := host.counts(); return h == 1 })

	// A click without a position falls back to TrayAnchor.
	host.click(platform.TrayClick{})
	waitFor(t, func() bool { s, _ := host.counts(); return len(s) == 2 })
	if shows, _ := host.counts(); shows[1] != host.anchor {
		t.Fatalf("panel shown at %+v, want TrayAnchor %+v", shows[1], host.anchor)
	}
	// The panel itself hears tray.click (it is the tray's window).
	waitFor(t, func() bool {
		for _, m := range host.posted {
			if m.Window == "panel" && bytes.Contains(m.Message, []byte(`"tray.click"`)) {
				return true
			}
		}
		return false
	})
}

// A panel is an ordinary window to the security model: it has no grant
// unless one names it, and its calls are attributed to it.
func TestTrayPanel_HasNoAuthorityByDefault(t *testing.T) {
	host := newPanelHost()
	rt, a := newPanelApp(t, host)
	if err := a.UseOfficialPlugins(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTray(app.TraySpec{Title: "x", Panel: "panel"}); err != nil {
		t.Fatal(err)
	}
	runAsync(t, a, host.ran)
	grantAll(t, rt) // grants "main" only
	caller, err := rt.CallerFor("panel")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "clipboard.read"})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Window != "panel" {
		t.Fatalf("panel clipboard.read without a grant: %v", err)
	}
}

func TestTrayPanel_ShowAndHideFromGo(t *testing.T) {
	host := newPanelHost()
	_, a := newPanelApp(t, host)
	if err := a.SetTray(app.TraySpec{Title: "x", Panel: "panel"}); err != nil {
		t.Fatal(err)
	}
	runAsync(t, a, host.ran)
	if err := a.ShowTrayPanel(); err != nil {
		t.Fatal(err)
	}
	if shows, _ := host.counts(); len(shows) != 1 || shows[0] != host.anchor {
		t.Fatalf("ShowTrayPanel shows %+v", shows)
	}
	if err := a.HideTrayPanel(); err != nil {
		t.Fatal(err)
	}
	if _, hides := host.counts(); hides != 1 {
		t.Fatalf("hides %d", hides)
	}
}

func TestTrayPanel_UnsupportedHost(t *testing.T) {
	host := newAccessoryHost() // no Panels
	rt, err := vitra.New(vitra.Config{AppID: "com.example.bar"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID: "com.example.bar", Assets: fstest.MapFS{"index.html": {Data: []byte("x")}},
		Host: host, Runtime: rt, Presentation: app.PresentationAccessory,
		Window: app.WindowOptions{ID: "panel", Kind: app.WindowKindPanel},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetTray(app.TraySpec{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	var unsupp *platform.ErrUnsupported
	if err := a.Run(context.Background()); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureWindowPanel {
		t.Fatalf("Run with a panel on a host without panels: %v", err)
	}
}

// A hidden panel is not enough to see or quit the app: it needs the tray.
func TestTrayPanel_AccessoryPanelNeedsTray(t *testing.T) {
	host := newPanelHost()
	_, a := newPanelApp(t, host)
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("Run with only a hidden panel and no tray succeeded")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal("condition not met")
}
