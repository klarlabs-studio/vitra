package app_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// accessoryHost is a tray host that records the presentation it was given.
type accessoryHost struct {
	*trayHost
	pmu          sync.Mutex
	presentation platform.Presentation
}

func newAccessoryHost() *accessoryHost {
	return &accessoryHost{trayHost: &trayHost{pluginHost: newPluginHost()}}
}

func (h *accessoryHost) Features() platform.FeatureSet {
	fs := h.trayHost.Features()
	fs[platform.FeaturePresentation] = platform.Support{Feature: platform.FeaturePresentation, Available: true}
	return fs
}

func (h *accessoryHost) SetPresentation(p platform.Presentation) error {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	h.presentation = p
	return nil
}

func (h *accessoryHost) got() platform.Presentation {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	return h.presentation
}

func newAccessoryApp(t *testing.T, host app.DesktopHost, window app.WindowOptions) *app.App {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.bar"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID: "com.example.bar", Assets: fstest.MapFS{"index.html": {Data: []byte("x")}},
		Host: host, Runtime: rt, Window: window, Presentation: app.PresentationAccessory,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func runAsync(t *testing.T, a *app.App, ran <-chan struct{}) {
	t.Helper()
	errc := make(chan error, 1)
	go func() { errc <- a.Run(context.Background()) }()
	select {
	case <-ran:
	case err := <-errc:
		t.Fatalf("Run: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	t.Cleanup(a.Quit)
}

// A menu bar app with no window: no Dock icon, nothing opened, and closing
// windows later does not quit it.
func TestAccessory_TrayOnlyApp(t *testing.T) {
	host := newAccessoryHost()
	a := newAccessoryApp(t, host, app.WindowOptions{})
	if err := a.SetTray(app.TraySpec{Title: "42%"}); err != nil {
		t.Fatal(err)
	}
	runAsync(t, a, host.ran)
	if got := host.got(); got != platform.PresentationAccessory {
		t.Fatalf("presentation %q", got)
	}
	if host.opened {
		t.Fatal("accessory app without a window opened one")
	}
	if err := a.OpenWindow(context.Background(), app.WindowOptions{ID: "settings"}); err != nil {
		t.Fatal(err)
	}
	if err := a.CloseWindow(context.Background(), "settings"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-host.quit:
		t.Fatal("closing the last window quit a menu bar app")
	case <-time.After(50 * time.Millisecond):
	}
}

// Without a window and without a tray the app would be invisible and could
// not be quit, so Run refuses it.
func TestAccessory_NeedsTrayOrWindow(t *testing.T) {
	host := newAccessoryHost()
	a := newAccessoryApp(t, host, app.WindowOptions{})
	err := a.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "SetTray") {
		t.Fatalf("Run without window or tray: %v", err)
	}
}

func TestAccessory_WithWindow(t *testing.T) {
	host := newAccessoryHost()
	a := newAccessoryApp(t, host, app.WindowOptions{ID: "main"})
	runAsync(t, a, host.ran)
	if !host.opened || host.got() != platform.PresentationAccessory {
		t.Fatalf("opened=%v presentation=%q", host.opened, host.got())
	}
}

func TestAccessory_UnsupportedHost(t *testing.T) {
	host := &trayHost{pluginHost: newPluginHost()} // no PresentationSetter
	a := newAccessoryApp(t, host, app.WindowOptions{ID: "main"})
	var unsupp *platform.ErrUnsupported
	if err := a.Run(context.Background()); !errors.As(err, &unsupp) || unsupp.Feature != platform.FeaturePresentation {
		t.Fatalf("Run on a host without presentation: %v", err)
	}
}

func TestPresentation_RejectsUnknown(t *testing.T) {
	_, err := app.New(app.Options{AppID: "com.example.bar", Host: newAccessoryHost(), Presentation: "floating"})
	var v *domain.ErrValidation
	if !errors.As(err, &v) {
		t.Fatalf("New with unknown presentation: %v", err)
	}
}
