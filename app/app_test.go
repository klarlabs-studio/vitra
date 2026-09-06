package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

type fakeHost struct {
	invoke  func(domain.WindowID, domain.Origin, []byte) []byte
	nav     func(domain.WindowID, string) bool
	opened  bool
	uri     string
	preload string
	quit    chan struct{}
	ran     chan struct{}
}

func (h *fakeHost) OS() platform.OS { return platform.OSLinux }
func (h *fakeHost) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {Feature: platform.FeatureWindowCreate, Available: true},
		platform.FeatureClipboard:    {Feature: platform.FeatureClipboard, Available: true},
	}
}
func (h *fakeHost) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.invoke = fn
}
func (h *fakeHost) SetNavPolicy(fn func(domain.WindowID, string) bool)      { h.nav = fn }
func (h *fakeHost) CreateWindow(context.Context, platform.WindowSpec) error { return nil }
func (h *fakeHost) Open(spec platform.WindowSpec, uri, preload string) error {
	h.opened = true
	h.uri = uri
	h.preload = preload
	return nil
}
func (h *fakeHost) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return nil
}
func (h *fakeHost) PostMessage(context.Context, domain.WindowID, []byte) error { return nil }
func (h *fakeHost) Eval(domain.WindowID, string) error                         { return nil }
func (h *fakeHost) CloseWindow(context.Context, domain.WindowID) error         { return nil }
func (h *fakeHost) ClipboardGet() (string, error)                              { return "clip", nil }
func (h *fakeHost) ClipboardSet(string) error                                  { return nil }
func (h *fakeHost) OpenFileDialog() (string, error)                            { return "/tmp/x", nil }
func (h *fakeHost) Run() error {
	close(h.ran)
	<-h.quit
	return nil
}
func (h *fakeHost) Quit() { close(h.quit) }

func TestApp_RunInvokeAndNavPolicy(t *testing.T) {
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	rt, err := vitra.New(vitra.Config{AppID: "com.vitra.app-test"})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := domain.NewCommandDefinition("demo.greet", "greet", "demo.greet")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterCommand(cmd, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		return map[string]any{"hello": input}, nil
	})); err != nil {
		t.Fatal(err)
	}
	grant, err := domain.NewCapabilityGrant(
		"demo", "demo", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "demo.greet"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}

	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}}
	application, err := app.New(app.Options{
		AppID:   "com.vitra.app-test",
		Assets:  assets,
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main", Width: 800, Height: 600},
	})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- application.Run(context.Background()) }()

	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	if !host.opened || !strings.HasPrefix(host.uri, "http://127.0.0.1:") {
		t.Fatalf("open uri=%q opened=%v", host.uri, host.opened)
	}
	if !strings.Contains(host.preload, "window.__vitra") {
		t.Fatal("preload missing bridge")
	}
	if host.nav == nil || !host.nav("main", host.uri+"/") {
		t.Fatal("expected local nav allowed")
	}
	if host.nav("main", "https://evil.example") {
		t.Fatal("external nav must be denied")
	}

	raw, _ := json.Marshal(map[string]any{
		"type": "invoke", "id": "1", "command": "demo.greet", "input": "Klar",
	})
	resp := host.invoke("main", domain.OriginPackagedLocal, raw)
	var got map[string]any
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("invoke failed: %s", resp)
	}

	application.Quit()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}

func TestApp_RequiresHostAndAssets(t *testing.T) {
	if _, err := app.New(app.Options{AppID: "x"}); err == nil {
		t.Fatal("expected host required")
	}
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	a, err := app.New(app.Options{AppID: "com.vitra.x", Host: host})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("expected assets required")
	}
}
