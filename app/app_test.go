package app_test

import (
	"context"
	"encoding/json"
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

type fakeHost struct {
	invoke    func(domain.WindowID, domain.Origin, []byte) []byte
	nav       func(domain.WindowID, string) bool
	onDestroy func(domain.WindowID)
	opened    bool
	opens     []platform.WindowSpec
	uri       string
	preload   string
	posted    []postedMsg
	quit      chan struct{}
	ran       chan struct{}
	closed    []domain.WindowID
	quitOnce  sync.Once
}

type postedMsg struct {
	Window  domain.WindowID
	Message []byte
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
	h.opens = append(h.opens, spec)
	h.uri = uri
	h.preload = preload
	return nil
}
func (h *fakeHost) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return nil
}
func (h *fakeHost) PostMessage(_ context.Context, id domain.WindowID, message []byte) error {
	cp := append([]byte(nil), message...)
	h.posted = append(h.posted, postedMsg{Window: id, Message: cp})
	return nil
}
func (h *fakeHost) Eval(domain.WindowID, string) error { return nil }
func (h *fakeHost) CloseWindow(_ context.Context, id domain.WindowID) error {
	h.closed = append(h.closed, id)
	if h.onDestroy != nil {
		h.onDestroy(id)
	}
	return nil
}
func (h *fakeHost) ClipboardGet() (string, error)                      { return "clip", nil }
func (h *fakeHost) ClipboardSet(string) error                          { return nil }
func (h *fakeHost) OpenFileDialog() (string, error)                    { return "/tmp/x", nil }
func (h *fakeHost) SaveFileDialog() (string, error)                    { return "/tmp/y", nil }
func (h *fakeHost) SetActionHandler(func(string))                      {}
func (h *fakeHost) SetDragDropHandler(func(domain.WindowID, []string)) {}
func (h *fakeHost) SetDestroyHandler(fn func(domain.WindowID))         { h.onDestroy = fn }
func (h *fakeHost) EnableDragDrop(domain.WindowID, bool) error         { return nil }
func (h *fakeHost) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return nil
}
func (h *fakeHost) SetTray(string, []platform.MenuItem) error { return nil }
func (h *fakeHost) ClearTray()                                {}
func (h *fakeHost) TrySingleInstance(string) (bool, func(), error) {
	return true, func() {}, nil
}
func (h *fakeHost) StartDeepLinkBridge(string, func(string)) (func(), error) {
	return func() {}, nil
}
func (h *fakeHost) ForwardToPrimary(string, []string) (bool, error) { return false, nil }
func (h *fakeHost) RegisterURLScheme(string, string, string) error  { return nil }
func (h *fakeHost) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error {
	return nil
}
func (h *fakeHost) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, nil
}
func (h *fakeHost) OpenURL(context.Context, string) error { return nil }
func (h *fakeHost) Run() error {
	close(h.ran)
	<-h.quit
	return nil
}
func (h *fakeHost) Quit() {
	h.quitOnce.Do(func() { close(h.quit) })
}

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
		"protocol": "1",
		"kind":     "invoke",
		"id":       "1",
		"payload": map[string]any{
			"command": "demo.greet",
			"input":   "Klar",
		},
	})
	resp := host.invoke("main", domain.OriginPackagedLocal, raw)
	var got map[string]any
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("invoke failed: %s", resp)
	}

	if _, err := rt.SubscribeEvent("sub-1", "demo.tick", "main"); err != nil {
		t.Fatal(err)
	}
	if err := application.Emit(context.Background(), "demo.tick", map[string]any{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if len(host.posted) != 1 {
		t.Fatalf("posted=%d", len(host.posted))
	}
	var ev map[string]any
	if err := json.Unmarshal(host.posted[0].Message, &ev); err != nil {
		t.Fatal(err)
	}
	if ev["type"] != "event" || ev["event"] != "demo.tick" {
		t.Fatalf("event msg: %+v", ev)
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

func TestApp_HelpersAndBadInvoke(t *testing.T) {
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}}
	application, err := app.New(app.Options{
		AppID:  "com.vitra.helpers",
		Assets: assets,
		Host:   host,
		Window: app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if application.Runtime() == nil {
		t.Fatal("runtime")
	}
	errCh := make(chan error, 1)
	go func() { errCh <- application.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("run timeout")
	}
	if application.Addr() == "" || !strings.Contains(application.DebugString(), "com.vitra.helpers") {
		t.Fatalf("addr/debug %q %q", application.Addr(), application.DebugString())
	}
	bad := host.invoke("main", domain.OriginPackagedLocal, []byte("{"))
	if !strings.Contains(string(bad), "bad_request") && !strings.Contains(string(bad), "invalid") {
		t.Fatalf("bad json: %s", bad)
	}
	wrongType, _ := json.Marshal(map[string]any{
		"protocol": "1", "kind": "event", "id": "1", "payload": map[string]any{},
	})
	resp := host.invoke("main", domain.OriginPackagedLocal, wrongType)
	if !strings.Contains(string(resp), "bad_request") && !strings.Contains(string(resp), "expected kind") {
		t.Fatalf("wrong type: %s", resp)
	}
	application.Quit()
	<-errCh
}

func TestApp_OpenAndCloseWindow(t *testing.T) {
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}}
	application, err := app.New(app.Options{
		AppID:  "com.vitra.multi",
		Assets: assets,
		Host:   host,
		Window: app.WindowOptions{ID: "main", Width: 800, Height: 600},
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- application.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("run timeout")
	}

	if err := application.OpenWindow(context.Background(), app.WindowOptions{ID: "aux", Title: "Aux", Width: 400, Height: 300}); err != nil {
		t.Fatal(err)
	}
	if err := application.OpenWindow(context.Background(), app.WindowOptions{ID: "aux"}); err == nil {
		t.Fatal("expected duplicate window error")
	}
	ids := application.Windows()
	if len(ids) != 2 {
		t.Fatalf("windows=%v", ids)
	}
	if len(host.opens) != 2 || host.opens[1].ID != "aux" {
		t.Fatalf("opens=%+v", host.opens)
	}

	if err := application.CloseWindow(context.Background(), "aux"); err != nil {
		t.Fatal(err)
	}
	if len(application.Windows()) != 1 {
		t.Fatalf("after close: %v", application.Windows())
	}
	// Closing last window should quit.
	if err := application.CloseWindow(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected quit after last window")
	}
	if len(host.closed) != 2 {
		t.Fatalf("closed=%v", host.closed)
	}
}

func TestApp_NativeDestroyQuitsLastWindow(t *testing.T) {
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}}
	application, err := app.New(app.Options{
		AppID:  "com.vitra.destroy",
		Assets: assets,
		Host:   host,
		Window: app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- application.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("run timeout")
	}
	if err := application.OpenWindow(context.Background(), app.WindowOptions{ID: "aux"}); err != nil {
		t.Fatal(err)
	}
	if host.onDestroy == nil {
		t.Fatal("expected destroy handler")
	}
	// Simulate titlebar close of main; aux remains — must not quit.
	host.onDestroy("main")
	if len(application.Windows()) != 1 {
		t.Fatalf("windows after main destroy: %v", application.Windows())
	}
	select {
	case <-host.quit:
		t.Fatal("quit must not fire while aux remains")
	case <-time.After(50 * time.Millisecond):
	}
	host.onDestroy("aux")
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected quit after last native destroy")
	}
}
