package app_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/platform"
)

// devtoolsHost records the devtools setting and whether it was applied
// before the first window opened (settings apply at webview creation).
type devtoolsHost struct {
	*fakeHost
	set         []bool
	beforeOpen  bool
}

func (h *devtoolsHost) SetDevTools(enabled bool) {
	h.set = append(h.set, enabled)
	h.beforeOpen = !h.opened
}

func (h *devtoolsHost) Open(spec platform.WindowSpec, uri, preload string) error {
	return h.fakeHost.Open(spec, uri, preload)
}

func runDevtoolsApp(t *testing.T, devtools bool) *devtoolsHost {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.devtools"})
	if err != nil {
		t.Fatal(err)
	}
	host := &devtoolsHost{fakeHost: &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}}
	application, err := app.New(app.Options{
		AppID:    "com.example.devtools",
		Assets:   fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Host:     host,
		Runtime:  rt,
		Window:   app.WindowOptions{ID: "main"},
		DevTools: devtools,
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = application.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	application.Quit()
	return host
}

// Devtools let anyone at the keyboard run script with the page's bridge
// access, so a shipped app must not have them unless it opts in.
func TestRun_DevToolsOffByDefault(t *testing.T) {
	t.Setenv(app.EnvDevTools, "")
	host := runDevtoolsApp(t, false)
	if len(host.set) != 1 || host.set[0] || !host.beforeOpen {
		t.Fatalf("SetDevTools calls=%v beforeOpen=%v", host.set, host.beforeOpen)
	}
}

func TestRun_DevToolsOptIn(t *testing.T) {
	t.Setenv(app.EnvDevTools, "")
	if host := runDevtoolsApp(t, true); len(host.set) != 1 || !host.set[0] {
		t.Fatalf("Options.DevTools: SetDevTools calls=%v", host.set)
	}
	t.Setenv(app.EnvDevTools, "1") // set by `vitra dev`
	if host := runDevtoolsApp(t, false); len(host.set) != 1 || !host.set[0] {
		t.Fatalf("%s=1: SetDevTools calls=%v", app.EnvDevTools, host.set)
	}
}
