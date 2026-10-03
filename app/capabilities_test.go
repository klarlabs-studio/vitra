package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// coreHost implements only platform.DesktopHost: windows, IPC, navigation
// policy and the UI loop. It has no optional capability, but its feature
// matrix claims every feature, so any failure below comes from the app
// detecting the missing interface, not from the matrix.
type coreHost struct {
	mu        sync.Mutex
	invoke    func(domain.WindowID, domain.Origin, []byte) []byte
	nav       func(domain.WindowID, string) bool
	onDestroy func(domain.WindowID)
	opened    []domain.WindowID
	posted    int
	ran       chan struct{}
	quit      chan struct{}
	quitOnce  sync.Once
}

var _ platform.DesktopHost = (*coreHost)(nil)

func newCoreHost() *coreHost {
	return &coreHost{ran: make(chan struct{}), quit: make(chan struct{})}
}

func (h *coreHost) OS() platform.OS { return platform.OSLinux }

func (h *coreHost) Features() platform.FeatureSet {
	fs := platform.FeatureSet{}
	for _, f := range []platform.Feature{
		platform.FeatureWindowCreate, platform.FeatureWindowNavigate, platform.FeatureWebViewMessage,
		platform.FeatureMenuBar, platform.FeatureTray, platform.FeatureDialogOpen, platform.FeatureDialogSave,
		platform.FeatureDialogMessage, platform.FeatureDialogOpenDirectory, platform.FeatureNotificationShow,
		platform.FeatureClipboard, platform.FeatureGlobalShortcut, platform.FeatureDragDrop,
		platform.FeatureWindowChrome, platform.FeatureOpenURL, platform.FeaturePathOpen,
	} {
		fs[f] = platform.Support{Feature: f, Available: true}
	}
	return fs
}

func (h *coreHost) CreateWindow(context.Context, platform.WindowSpec) error { return nil }
func (h *coreHost) NavigateWindow(context.Context, domain.WindowID, domain.Origin) error {
	return nil
}
func (h *coreHost) PostMessage(context.Context, domain.WindowID, []byte) error {
	h.mu.Lock()
	h.posted++
	h.mu.Unlock()
	return nil
}
func (h *coreHost) postedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.posted
}
func (h *coreHost) CloseWindow(context.Context, domain.WindowID) error { return nil }
func (h *coreHost) Open(spec platform.WindowSpec, _, _ string) error {
	h.mu.Lock()
	h.opened = append(h.opened, spec.ID)
	h.mu.Unlock()
	return nil
}
func (h *coreHost) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.invoke = fn
}
func (h *coreHost) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.nav = fn }
func (h *coreHost) SetDestroyHandler(fn func(domain.WindowID))         { h.onDestroy = fn }
func (h *coreHost) Run() error {
	close(h.ran)
	<-h.quit
	return nil
}
func (h *coreHost) Quit() { h.quitOnce.Do(func() { close(h.quit) }) }

// A host with only the core interface runs an app: it opens the primary
// window, enforces the navigation policy and answers calls from the page.
func TestCoreHost_RunsApp(t *testing.T) {
	host := newCoreHost()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.core"})
	if err != nil {
		t.Fatal(err)
	}
	def, _ := domain.NewCommandDefinition("demo.ping", "ping", "demo.ping")
	if err := rt.RegisterCommand(def, domain.CommandExecutorFunc(func(context.Context, domain.CommandName, any) (any, error) {
		return "pong", nil
	})); err != nil {
		t.Fatal(err)
	}
	g, _ := domain.NewCapabilityGrant("ping", "ping", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{Name: "demo.ping"}})
	if err := rt.RegisterGrant(g); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID:   "com.example.core",
		Assets:  fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Host:    host,
		Runtime: rt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.UseOfficialPlugins(context.Background()); err != nil {
		t.Fatalf("UseOfficialPlugins on a core-only host: %v", err)
	}
	go func() { _ = a.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	t.Cleanup(a.Quit)

	if len(host.opened) != 1 || host.opened[0] != "main" {
		t.Fatalf("opened windows = %v", host.opened)
	}
	if host.nav("main", "https://evil.example/") {
		t.Fatal("navigation to a remote page allowed")
	}
	if host.invoke == nil {
		t.Fatal("invoke handler not installed")
	}
	// The page bridge needs the window's sender token, which the core host
	// received in the preload; the kernel path is covered by rt.Invoke.
	caller, _ := rt.CallerFor("main")
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "demo.ping"})
	if err != nil || res.Output != "pong" {
		t.Fatalf("demo.ping = %v, %v", res, err)
	}
	if err := a.Emit(context.Background(), "demo.tick", nil); err != nil {
		t.Fatalf("Emit: %v", err)
	}
}

// Every optional capability the official commands need is detected on the
// host; a host without it fails each call with *platform.ErrUnsupported
// naming the feature, never with a silent success.
func TestCoreHost_OptionalCapabilitiesAreUnsupported(t *testing.T) {
	host := newCoreHost()
	rt, _ := runPluginAppOn(t, host, host.ran)
	grantAll(t, rt)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := domain.NewCapabilityGrant("paths", "open the temp dir", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "path.open", PathScope: &domain.PathScope{Allow: []string{dir + "/**"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(g); err != nil {
		t.Fatal(err)
	}

	items := []any{map[string]any{"menu": "File", "id": "app.save", "label": "Save"}}
	cases := []struct {
		cmd     domain.CommandName
		input   any
		rp      string
		feature platform.Feature
	}{
		{"clipboard.read", nil, "", platform.FeatureClipboard},
		{"clipboard.write", "x", "", platform.FeatureClipboard},
		{"dialog.open", map[string]any{}, "", platform.FeatureDialogOpen},
		{"dialog.open", map[string]any{"multiple": true}, "", platform.FeatureDialogOpen},
		{"dialog.save", map[string]any{}, "", platform.FeatureDialogSave},
		{"dialog.openDirectory", map[string]any{}, "", platform.FeatureDialogOpenDirectory},
		{"dialog.message", "hi", "", platform.FeatureDialogMessage},
		{"notifications.show", "hi", "", platform.FeatureNotificationShow},
		{"menu.set", items, "", platform.FeatureMenuBar},
		{"menu.clear", nil, "", platform.FeatureMenuBar},
		{"tray.set", map[string]any{"tooltip": "t", "items": items}, "", platform.FeatureTray},
		{"tray.clear", nil, "", platform.FeatureTray},
		{"shortcut.register", map[string]any{"accelerator": "Ctrl+K", "action": "app.find"}, "", platform.FeatureGlobalShortcut},
		{"shortcut.unregister", map[string]any{"accelerator": "Ctrl+K"}, "", platform.FeatureGlobalShortcut},
		{"dragdrop.receive", map[string]any{"window": "main", "enabled": true}, "", platform.FeatureDragDrop},
		{"window.focus", "main", "", platform.FeatureWindowChrome},
		{"window.blur", "main", "", platform.FeatureWindowChrome},
		{"window.getChrome", "main", "", platform.FeatureWindowChrome},
		{"window.setTitle", map[string]any{"id": "main", "title": "T"}, "", platform.FeatureWindowChrome},
		{"browser.open", "https://example.com/", "", platform.FeatureOpenURL},
		{"path.open", file, file, platform.FeaturePathOpen},
	}
	for _, c := range cases {
		out, err := call(t, rt, c.cmd, c.input, c.rp)
		var unsupp *platform.ErrUnsupported
		if !errors.As(err, &unsupp) || unsupp.Feature != c.feature {
			t.Errorf("%s(%v) = %v, %v; want *platform.ErrUnsupported for %s", c.cmd, c.input, out, err, c.feature)
		}
	}

	// Commands that need only the core still work.
	if _, err := call(t, rt, "window.create", map[string]any{"id": "aux"}, ""); err != nil {
		t.Fatalf("window.create on a core host: %v", err)
	}
	if _, err := call(t, rt, "window.close", "aux", ""); err != nil {
		t.Fatalf("window.close on a core host: %v", err)
	}
	if out, err := call(t, rt, "os.info", nil, ""); err != nil {
		t.Fatalf("os.info on a core host = %v, %v", out, err)
	}
}

// actionHost adds menus (and with them action reporting) to the core.
type actionHost struct {
	*coreHost
	action func(string)
	menus  [][]platform.MenuItem
}

var _ platform.MenuBar = (*actionHost)(nil)

func (h *actionHost) SetActionHandler(fn func(string)) { h.action = fn }
func (h *actionHost) SetMenuBar(_ domain.WindowID, items []platform.MenuItem) error {
	h.menus = append(h.menus, items)
	return nil
}

// Implementing one optional interface enables exactly that capability.
func TestOptionalCapability_DetectedPerInterface(t *testing.T) {
	host := &actionHost{coreHost: newCoreHost()}
	rt, _ := runPluginAppOn(t, host, host.ran)
	grantAll(t, rt)
	if _, err := rt.SubscribeEvent("s-menu", "menu.action", "main"); err != nil {
		t.Fatal(err)
	}

	if _, err := call(t, rt, "menu.set", []any{map[string]any{"menu": "File", "id": "app.save", "label": "Save"}}, ""); err != nil {
		t.Fatalf("menu.set on a MenuBar host: %v", err)
	}
	if len(host.menus) != 1 || host.menus[0][0].ID != "app.save" {
		t.Fatalf("menus = %+v", host.menus)
	}
	if host.action == nil {
		t.Fatal("action handler not installed on a MenuBar host")
	}
	before := host.postedCount()
	host.action("app.save")
	if host.postedCount() == before {
		t.Fatal("menu activation was not emitted to the page")
	}

	_, err := call(t, rt, "tray.set", map[string]any{"tooltip": "t"}, "")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureTray {
		t.Fatalf("tray.set on a host without platform.Tray: %v", err)
	}
}
