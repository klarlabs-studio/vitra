package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official"
)

// pluginHost records what the official plugins ask of the native host.
type pluginHost struct {
	*fakeHost
	mu      sync.Mutex
	menus   map[domain.WindowID][]platform.MenuItem
	action  func(string)
	drop    func(domain.WindowID, []string)
	cleared bool
}

func newPluginHost() *pluginHost {
	return &pluginHost{
		fakeHost: &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})},
		menus:    map[domain.WindowID][]platform.MenuItem{},
	}
}

func (h *pluginHost) Features() platform.FeatureSet {
	fs := platform.FeatureSet{}
	for _, f := range []platform.Feature{
		platform.FeatureWindowCreate, platform.FeatureClipboard, platform.FeatureDialogOpen,
		platform.FeatureMenuBar, platform.FeatureTray,
	} {
		fs[f] = platform.Support{Feature: f, Available: true}
	}
	return fs
}

func (h *pluginHost) SetActionHandler(fn func(string)) { h.action = fn }

func (h *pluginHost) SetDragDropHandler(fn func(domain.WindowID, []string)) { h.drop = fn }

func (h *pluginHost) ClearTray() { h.cleared = true }

func (h *pluginHost) SetMenuBar(id domain.WindowID, items []platform.MenuItem) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.menus[id] = items
	return nil
}

func runPluginApp(t *testing.T, plugins ...plugin.Plugin) (*vitra.Runtime, *pluginHost, *app.App) {
	t.Helper()
	host := newPluginHost()
	rt, a := runPluginAppOn(t, host, host.ran, plugins...)
	return rt, host, a
}

// runPluginAppOn runs an app with the official plugins on host; ran closes
// once the host's Run has started.
func runPluginAppOn(t *testing.T, host app.DesktopHost, ran <-chan struct{}, plugins ...plugin.Plugin) (*vitra.Runtime, *app.App) {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.plugins"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID:   "com.example.plugins",
		Assets:  fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.UseOfficialPlugins(context.Background(), plugins...); err != nil {
		t.Fatal(err)
	}
	go func() { _ = a.Run(context.Background()) }()
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	t.Cleanup(a.Quit)
	return rt, a
}

func grantAll(t *testing.T, rt *vitra.Runtime) {
	t.Helper()
	var perms []domain.PermissionSpec
	for _, p := range official.All() {
		for _, name := range p.Manifest().Permissions {
			spec := domain.PermissionSpec{Name: name}
			if name == "fs.read" || name == "fs.write" || name == "path.open" {
				spec.PathScope = &domain.PathScope{Allow: []string{"/tmp/**"}}
			}
			perms = append(perms, spec)
		}
	}
	g, err := domain.NewCapabilityGrant("all", "every official permission",
		[]domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal}, perms)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(g); err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, rt *vitra.Runtime, cmd domain.CommandName, input any, rp string) (any, error) {
	t.Helper()
	caller, err := rt.CallerFor("main")
	if err != nil {
		t.Fatal(err)
	}
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: cmd, Input: input, ResourcePath: rp})
	if err != nil {
		return nil, err
	}
	return res.Output, nil
}

// One call binds every official command to the host, and grants nothing.
func TestUseOfficialPlugins_BindsEveryCommandAndGrantsNothing(t *testing.T) {
	rt, _, _ := runPluginApp(t)

	var commands []domain.CommandName
	for _, p := range official.All() {
		c, err := p.Contribute()
		if err != nil {
			t.Fatal(err)
		}
		for _, cmd := range c.Commands {
			commands = append(commands, cmd.Name())
		}
	}
	if len(commands) == 0 {
		t.Fatal("no official commands found")
	}

	// Without a grant, nothing is allowed.
	for _, cmd := range commands {
		_, err := call(t, rt, cmd, nil, "/tmp/x")
		var denied *domain.ErrDenied
		if !errors.As(err, &denied) {
			t.Fatalf("%s without a grant: %v, want a denial", cmd, err)
		}
	}

	// With every permission granted, every command reaches an executor.
	grantAll(t, rt)
	for _, cmd := range commands {
		_, err := call(t, rt, cmd, nil, "/tmp/x")
		var nf *domain.ErrNotFound
		if errors.As(err, &nf) && nf.Entity == "command executor" {
			t.Errorf("%s has no executor", cmd)
		}
	}
}

func TestUseOfficialPlugins_CallsTheHost(t *testing.T) {
	rt, host, a := runPluginApp(t)
	grantAll(t, rt)

	if out, err := call(t, rt, "clipboard.read", nil, ""); err != nil || out != "clip" {
		t.Fatalf("clipboard.read = %v, %v", out, err)
	}
	out, err := call(t, rt, "dialog.open", map[string]any{"title": "Pick"}, "")
	if paths, _ := out.([]string); err != nil || len(paths) != 1 || paths[0] != "/tmp/x" {
		t.Fatalf("dialog.open = %v, %v", out, err)
	}
	if _, err := call(t, rt, "menu.set", []any{map[string]any{"menu": "File", "id": "app.save", "label": "Save"}}, ""); err != nil {
		t.Fatalf("menu.set: %v", err)
	}
	host.mu.Lock()
	got := host.menus["main"]
	host.mu.Unlock()
	if len(got) != 1 || got[0].ID != "app.save" || got[0].Menu != "File" {
		t.Fatalf("menu bar on main = %+v", got)
	}
	if _, err := call(t, rt, "tray.clear", nil, ""); err != nil || !host.cleared {
		t.Fatalf("tray.clear: %v (cleared=%v)", err, host.cleared)
	}
	out, err = call(t, rt, "window.create", map[string]any{"id": "aux", "title": "Aux"}, "")
	if err != nil {
		t.Fatalf("window.create: %v", err)
	}
	if ids := a.Windows(); len(ids) != 2 {
		t.Fatalf("windows after window.create = %v (result %v)", ids, out)
	}
}

// Native activations and file drops reach subscribed windows as events.
func TestUseOfficialPlugins_EmitsNativeEvents(t *testing.T) {
	rt, host, _ := runPluginApp(t)
	for _, ev := range []domain.EventName{"menu.action", "dragdrop.drop"} {
		if _, err := rt.SubscribeEvent(domain.SubscriptionID("s-"+string(ev)), ev, "main"); err != nil {
			t.Fatal(err)
		}
	}
	if host.action == nil || host.drop == nil {
		t.Fatal("native action or drop handler not installed")
	}
	host.action("app.save")
	host.drop("main", []string{"/tmp/a.md"})

	events := map[string]map[string]any{}
	for _, m := range host.posted {
		var msg struct {
			Event   string         `json:"event"`
			Payload map[string]any `json:"payload"`
		}
		if err := json.Unmarshal(m.Message, &msg); err == nil {
			events[msg.Event] = msg.Payload
		}
	}
	if events["menu.action"]["id"] != "app.save" {
		t.Fatalf("menu.action = %v", events["menu.action"])
	}
	if paths, _ := events["dragdrop.drop"]["paths"].([]any); len(paths) != 1 || events["dragdrop.drop"]["window"] != "main" {
		t.Fatalf("dragdrop.drop = %v", events["dragdrop.drop"])
	}
}

// Apps can take only the plugins they need.
func TestUseOfficialPlugins_Subset(t *testing.T) {
	rt, _, _ := runPluginApp(t, official.Clipboard())
	grantAll(t, rt)
	if _, err := call(t, rt, "clipboard.read", nil, ""); err != nil {
		t.Fatalf("clipboard.read: %v", err)
	}
	_, err := call(t, rt, "dialog.open", nil, "")
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialCommandMissing {
		t.Fatalf("dialog.open without the dialog plugin: %v", err)
	}
}

// multiPickHost is a pluginHost whose open dialog can select several files.
type multiPickHost struct {
	*pluginHost
	gotOpts platform.DialogFileOptions
}

func (h *multiPickHost) OpenFilesDialog(opts platform.DialogFileOptions) ([]string, error) {
	h.gotOpts = opts
	return []string{"/etc/picked-a.txt", "/etc/picked-b.txt"}, nil
}

func TestDialogOpen_MultipleReturnsEverySelectedPath(t *testing.T) {
	host := &multiPickHost{pluginHost: newPluginHost()}
	rt, _ := runPluginAppOn(t, host, host.ran)
	grantAll(t, rt)

	out, err := call(t, rt, "dialog.open", map[string]any{"title": "Pick", "multiple": true}, "")
	paths, _ := out.([]string)
	if err != nil || len(paths) != 2 || paths[0] != "/etc/picked-a.txt" || paths[1] != "/etc/picked-b.txt" {
		t.Fatalf("dialog.open multiple = %v, %v", out, err)
	}
	if !host.gotOpts.Multiple || host.gotOpts.Title != "Pick" {
		t.Fatalf("host got options %+v", host.gotOpts)
	}

	// Without multiple, the single-file dialog is used, as before.
	out, err = call(t, rt, "dialog.open", map[string]any{"title": "Pick"}, "")
	if paths, _ := out.([]string); err != nil || len(paths) != 1 || paths[0] != "/tmp/x" {
		t.Fatalf("dialog.open = %v, %v", out, err)
	}

	// Picking files grants nothing: the picked paths lie outside the fs.read
	// scope, so reading them is still denied.
	for _, p := range paths {
		_, err := call(t, rt, "fs.read", p, p)
		var denied *domain.ErrDenied
		if !errors.As(err, &denied) {
			t.Fatalf("fs.read %s after picking it: %v, want denied", p, err)
		}
	}
}

func TestDialogOpen_MultipleUnsupportedByHost(t *testing.T) {
	rt, _, _ := runPluginApp(t)
	grantAll(t, rt)

	out, err := call(t, rt, "dialog.open", map[string]any{"multiple": true}, "")
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDialogOpen {
		t.Fatalf("dialog.open multiple on a single-pick host = %v, %v; want ErrUnsupported", out, err)
	}
}

type customPlugin struct{}

func (customPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{ID: "example.custom", Name: "Custom", Version: plugin.SemVer{Major: 1}}
}
func (customPlugin) Contribute() (plugin.Contribution, error) { return plugin.Contribution{}, nil }

func TestUseOfficialPlugins_RejectsOtherPlugins(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.plugins"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID: "com.example.plugins", Runtime: rt, Host: newPluginHost(),
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Window: app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.UseOfficialPlugins(context.Background(), customPlugin{}); err == nil {
		t.Fatal("a non-official plugin was accepted")
	}
}
