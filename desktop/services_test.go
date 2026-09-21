package desktop_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/null"
)

type allowAll struct{}

func (allowAll) Authorize(domain.Caller, domain.PermissionName, string) domain.Decision {
	return domain.Decision{Allowed: true, Reason: "test allow"}
}

type denyAll struct{}

func (denyAll) Authorize(_ domain.Caller, perm domain.PermissionName, _ string) domain.Decision {
	return domain.Decision{Permission: perm, Code: domain.DenialNoGrant, Reason: "denied in test"}
}

type featureHost struct {
	*null.Host
	extra platform.FeatureSet
}

func (h *featureHost) Features() platform.FeatureSet {
	fs := h.Host.Features()
	for k, v := range h.extra {
		fs[k] = v
	}
	return fs
}

func withFeatures(base platform.OS, features ...platform.Feature) *featureHost {
	extra := platform.FeatureSet{}
	for _, f := range features {
		extra[f] = platform.Support{Feature: f, Available: true}
	}
	return &featureHost{Host: null.New(base), extra: extra}
}

func TestMenuService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)

	denied := &desktop.MenuService{Gateway: denyAll{}, Host: host}
	err := denied.SetMenu(context.Background(), caller, nil)
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	svc := &desktop.MenuService{Gateway: allowAll{}, Host: host}
	err = svc.SetMenu(context.Background(), caller, []desktop.MenuItem{{ID: "quit", Label: "Quit"}})
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureMenuBar {
		t.Fatalf("expected unsupported menu, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeatureMenuBar)
	called := false
	cleared := false
	ok := &desktop.MenuService{
		Gateway: allowAll{},
		Host:    okHost,
		OnSet: func(ctx context.Context, items []desktop.MenuItem) error {
			called = true
			return nil
		},
		OnClear: func(ctx context.Context) error {
			cleared = true
			return nil
		},
	}
	if err := ok.SetMenu(context.Background(), caller, []desktop.MenuItem{{ID: "quit", Label: "Quit"}}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected OnSet")
	}
	if err := ok.ClearMenu(context.Background(), caller); err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("expected OnClear")
	}
	items, err := desktop.ParseMenuItems([]any{
		map[string]any{"id": "app.quit", "label": "Quit", "menu": "File", "shortcut": "Ctrl+Q"},
		map[string]any{"id": "help.about", "label": "About", "menu": "Help"},
	})
	if err != nil || len(items) != 2 || items[0].ID != "app.quit" || items[0].Menu != "File" || items[0].Shortcut != "Ctrl+Q" || items[1].Label != "About" {
		t.Fatalf("parse: %+v err=%v", items, err)
	}
	wrapped, err := desktop.ParseMenuItems(map[string]any{"items": []any{
		map[string]any{"id": "x", "label": "X"},
	}})
	if err != nil || len(wrapped) != 1 || wrapped[0].ID != "x" {
		t.Fatalf("parse wrapped: %+v err=%v", wrapped, err)
	}
	if _, err := desktop.ParseMenuItems(map[string]any{"id": "x"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTrayDialogClipboardShortcutSingleInstance(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := withFeatures(platform.OSLinux,
		platform.FeatureTray,
		platform.FeatureDialogOpen,
		platform.FeatureDialogSave,
		platform.FeatureClipboard,
		platform.FeatureGlobalShortcut,
		platform.FeatureSingleInstance,
	)
	ctx := context.Background()

	cleared := false
	trays := &desktop.TrayService{
		Gateway: allowAll{}, Host: host,
		OnSet: func(context.Context, string, []desktop.MenuItem) error { return nil },
		OnClear: func(context.Context) error {
			cleared = true
			return nil
		},
	}
	if err := trays.SetTray(ctx, caller, "Vitra", nil); err != nil {
		t.Fatal(err)
	}
	if err := trays.ClearTray(ctx, caller); err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("expected OnClear")
	}
	tip, items, err := desktop.ParseTraySet(map[string]any{
		"tooltip": "Demo",
		"items": []any{
			map[string]any{"id": "tray.quit", "label": "Quit"},
		},
	})
	if err != nil || tip != "Demo" || len(items) != 1 || items[0].ID != "tray.quit" {
		t.Fatalf("parse tray: tip=%q items=%+v err=%v", tip, items, err)
	}
	tip, items, err = desktop.ParseTraySet([]any{map[string]any{"id": "a", "label": "A"}})
	if err != nil || tip != "" || len(items) != 1 || items[0].ID != "a" {
		t.Fatalf("parse tray array: tip=%q items=%+v err=%v", tip, items, err)
	}

	dlg := &desktop.DialogService{
		Gateway: allowAll{}, Host: host,
		OnOpen: func(_ context.Context, opts platform.DialogFileOptions) ([]string, error) {
			if opts.Title != "Open me" || len(opts.Filters) != 1 || opts.Filters[0].Name != "Images" {
				t.Fatalf("open opts: %+v", opts)
			}
			return []string{"/tmp/a"}, nil
		},
		OnSave: func(_ context.Context, opts platform.DialogFileOptions) (string, error) {
			if opts.DefaultPath != "/tmp/out.txt" {
				t.Fatalf("save opts: %+v", opts)
			}
			return "/tmp/b", nil
		},
	}
	paths, err := dlg.OpenFile(ctx, caller, platform.DialogFileOptions{
		Title:   "Open me",
		Filters: []platform.FileFilter{{Name: "Images", Extensions: []string{"png"}}},
	})
	if err != nil || len(paths) != 1 {
		t.Fatalf("open: %v %v", paths, err)
	}
	save, err := dlg.SaveFile(ctx, caller, platform.DialogFileOptions{DefaultPath: "/tmp/out.txt"})
	if err != nil || save != "/tmp/b" {
		t.Fatalf("save: %v %v", save, err)
	}
	bare := &desktop.DialogService{Gateway: allowAll{}, Host: host}
	if _, err := bare.OpenFile(ctx, caller, platform.DialogFileOptions{}); err == nil {
		t.Fatal("expected missing open adapter")
	}
	if _, err := bare.SaveFile(ctx, caller, platform.DialogFileOptions{}); err == nil {
		t.Fatal("expected missing save adapter")
	}
	parsed := desktop.ParseDialogFileOptions(map[string]any{
		"title":       "T",
		"defaultPath": "/home",
		"filters": []any{
			map[string]any{"name": "Docs", "extensions": []any{"pdf", "txt"}},
		},
	})
	if parsed.Title != "T" || parsed.DefaultPath != "/home" || len(parsed.Filters) != 1 ||
		parsed.Filters[0].Name != "Docs" || len(parsed.Filters[0].Extensions) != 2 {
		t.Fatalf("parse: %+v", parsed)
	}
	if got := desktop.ParseDialogFileOptions(nil); got.Title != "" || len(got.Filters) != 0 {
		t.Fatalf("nil parse: %+v", got)
	}
	dirHost := withFeatures(platform.OSLinux, platform.FeatureDialogOpenDirectory)
	dirDlg := &desktop.DialogService{
		Gateway: allowAll{}, Host: dirHost,
		OnOpenDirectory: func(context.Context) (string, error) { return "/tmp/d", nil },
	}
	dir, err := dirDlg.OpenDirectory(ctx, caller)
	if err != nil || dir != "/tmp/d" {
		t.Fatalf("opendir: %v %v", dir, err)
	}
	bareDir := &desktop.DialogService{Gateway: allowAll{}, Host: dirHost}
	if _, err := bareDir.OpenDirectory(ctx, caller); err == nil {
		t.Fatal("expected missing directory adapter")
	}
	msgHost := withFeatures(platform.OSLinux, platform.FeatureDialogMessage)
	msgDlg := &desktop.DialogService{
		Gateway: allowAll{}, Host: msgHost,
		OnMessage: func(_ context.Context, title, message, kind string) (bool, error) {
			if title != "T" || message != "M" || kind != "confirm" {
				t.Fatalf("args %q %q %q", title, message, kind)
			}
			return true, nil
		},
	}
	confirmed, err := msgDlg.Message(ctx, caller, "T", "M", "confirm")
	if err != nil || !confirmed {
		t.Fatalf("message: %v %v", confirmed, err)
	}
	if _, err := msgDlg.Message(ctx, caller, "T", "", "info"); err == nil {
		t.Fatal("expected empty message validation")
	}
	if _, err := msgDlg.Message(ctx, caller, "T", "M", "warn"); err == nil {
		t.Fatal("expected bad kind validation")
	}
	bareMsg := &desktop.DialogService{Gateway: allowAll{}, Host: msgHost}
	if _, err := bareMsg.Message(ctx, caller, "T", "M", "info"); err == nil {
		t.Fatal("expected missing message adapter")
	}

	clip := &desktop.ClipboardService{
		Gateway: allowAll{}, Host: host,
		OnRead:  func(context.Context) (string, error) { return "hi", nil },
		OnWrite: func(context.Context, string) error { return nil },
	}
	text, err := clip.Read(ctx, caller)
	if err != nil || text != "hi" {
		t.Fatalf("read: %v %v", text, err)
	}
	if err := clip.Write(ctx, caller, "yo"); err != nil {
		t.Fatal(err)
	}

	unregistered := ""
	sc := &desktop.ShortcutService{
		Gateway: allowAll{}, Host: host,
		OnRegister: func(context.Context, string, string) error { return nil },
		OnUnregister: func(_ context.Context, accelerator string) error {
			unregistered = accelerator
			return nil
		},
	}
	if err := sc.Register(ctx, caller, "Ctrl+Shift+P", "app.palette"); err != nil {
		t.Fatal(err)
	}
	if err := sc.Register(ctx, caller, "", "app.palette"); err == nil {
		t.Fatal("expected empty accelerator validation")
	}
	if err := sc.Register(ctx, caller, "Ctrl+Shift+P", ""); err == nil {
		t.Fatal("expected empty action validation")
	}
	if err := sc.Unregister(ctx, caller, "Ctrl+Shift+P"); err != nil {
		t.Fatal(err)
	}
	if unregistered != "Ctrl+Shift+P" {
		t.Fatalf("unregister: %q", unregistered)
	}
	if err := sc.Unregister(ctx, caller, ""); err == nil {
		t.Fatal("expected empty accelerator validation on unregister")
	}
	acc, act, err := desktop.ParseShortcutRegister(map[string]any{
		"accelerator": "Ctrl+Shift+Q", "action": "app.quit",
	})
	if err != nil || acc != "Ctrl+Shift+Q" || act != "app.quit" {
		t.Fatalf("parse: %s %s err=%v", acc, act, err)
	}
	_, _, err = desktop.ParseShortcutRegister(map[string]any{"accelerator": "Ctrl+A"})
	if err == nil {
		t.Fatal("expected missing action validation")
	}
	acc, err = desktop.ParseShortcutUnregister(map[string]any{"accelerator": "Ctrl+Shift+Q"})
	if err != nil || acc != "Ctrl+Shift+Q" {
		t.Fatalf("parse unregister object: %s err=%v", acc, err)
	}
	acc, err = desktop.ParseShortcutUnregister("Ctrl+B")
	if err != nil || acc != "Ctrl+B" {
		t.Fatalf("parse unregister string: %s err=%v", acc, err)
	}
	_, err = desktop.ParseShortcutUnregister(map[string]any{})
	if err == nil {
		t.Fatal("expected missing accelerator validation")
	}

	si := &desktop.SingleInstanceService{
		Gateway: allowAll{}, Host: host,
		OnLock: func(context.Context) (bool, error) { return true, nil },
	}
	ok, err := si.Acquire(ctx, caller)
	if err != nil || !ok {
		t.Fatalf("acquire: %v %v", ok, err)
	}

	deniedApp := &desktop.AppService{Gateway: denyAll{}}
	err = deniedApp.Quit(ctx, caller)
	var d *domain.ErrDenied
	if !errors.As(err, &d) {
		t.Fatalf("expected app.quit denial, got %v", err)
	}
	quitCalled := false
	appSvc := &desktop.AppService{
		Gateway: allowAll{},
		OnQuit: func(context.Context) error {
			quitCalled = true
			return nil
		},
	}
	if err := appSvc.Quit(ctx, caller); err != nil || !quitCalled {
		t.Fatalf("quit: called=%v err=%v", quitCalled, err)
	}
	if err := (&desktop.AppService{Gateway: allowAll{}}).Quit(ctx, caller); err == nil {
		t.Fatal("expected missing quit adapter")
	}
}

func TestClipboard_DeniedWithoutGrant(t *testing.T) {
	host := null.New(platform.OSDarwin)
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	_, err := (&desktop.ClipboardService{Gateway: denyAll{}, Host: host}).Read(context.Background(), caller)
	var d *domain.ErrDenied
	if !errors.As(err, &d) {
		t.Fatalf("expected denial, got %v", err)
	}
}

func TestDeepLink_RequiresPatternMatch(t *testing.T) {
	h := withFeatures(platform.OSWindows, platform.FeatureDeepLink)
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	svc := &desktop.DeepLinkService{
		Gateway:  allowAll{},
		Host:     h,
		Patterns: []domain.DeepLinkPattern{{Scheme: "vitra"}},
	}
	ok, err := svc.Handle(caller, "vitra://open/x")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, err := svc.Handle(caller, "https://evil.example"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := (&desktop.DeepLinkService{Gateway: denyAll{}, Host: h}).Handle(caller, "vitra://open/x"); err == nil {
		t.Fatal("expected denial")
	}
}

func TestAuthorize_NilGateway(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	if err := (&desktop.MenuService{Host: null.New(platform.OSLinux)}).SetMenu(context.Background(), caller, nil); err == nil {
		t.Fatal("expected gateway required")
	}
}

func TestDragDropService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)
	ctx := context.Background()

	denied := &desktop.DragDropService{Gateway: denyAll{}, Host: host}
	err := denied.Enable(ctx, caller, "main", true)
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	bare := &desktop.DragDropService{Gateway: allowAll{}, Host: host}
	err = bare.Enable(ctx, caller, "main", true)
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureDragDrop {
		t.Fatalf("expected unsupported drag_drop, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeatureDragDrop)
	called := false
	ok := &desktop.DragDropService{
		Gateway: allowAll{},
		Host:    okHost,
		OnEnable: func(_ context.Context, window domain.WindowID, enabled bool) error {
			called = true
			if window != "main" || !enabled {
				t.Fatalf("window=%s enabled=%v", window, enabled)
			}
			return nil
		},
	}
	if err := ok.Enable(ctx, caller, "main", true); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected OnEnable")
	}
	if err := ok.Enable(ctx, caller, "", true); err == nil {
		t.Fatal("expected empty window validation")
	}
	missing := &desktop.DragDropService{Gateway: allowAll{}, Host: okHost}
	if err := missing.Enable(ctx, caller, "main", true); err == nil {
		t.Fatal("expected missing adapter")
	}
	win, enabled, err := desktop.ParseDragDropEnable(map[string]any{"window": "aux", "enabled": false})
	if err != nil || win != "aux" || enabled {
		t.Fatalf("parse: %s %v err=%v", win, enabled, err)
	}
	win, enabled, err = desktop.ParseDragDropEnable(true)
	if err != nil || win != "main" || !enabled {
		t.Fatalf("parse bool: %s %v err=%v", win, enabled, err)
	}
}

func TestWindowService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)
	ctx := context.Background()
	chrome := platform.WindowChrome{Title: "Vitra", Width: 800, Height: 600, AlwaysOnTop: true}

	denied := &desktop.WindowService{Gateway: denyAll{}, Host: host}
	err := denied.Apply(ctx, caller, "main", chrome)
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	bare := &desktop.WindowService{Gateway: allowAll{}, Host: host}
	err = bare.Apply(ctx, caller, "main", chrome)
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureWindowChrome {
		t.Fatalf("expected unsupported window.chrome, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeatureWindowChrome)
	var got platform.WindowChrome
	ok := &desktop.WindowService{
		Gateway: allowAll{},
		Host:    okHost,
		OnApply: func(_ context.Context, window domain.WindowID, chrome platform.WindowChrome) error {
			if window != "main" {
				t.Fatalf("window=%s", window)
			}
			got = chrome
			return nil
		},
	}
	if err := ok.Apply(ctx, caller, "main", chrome); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Vitra" || got.Width != 800 || !got.AlwaysOnTop {
		t.Fatalf("chrome=%+v", got)
	}
	if err := ok.Apply(ctx, caller, "", chrome); err == nil {
		t.Fatal("expected empty window validation")
	}
	if err := ok.Apply(ctx, caller, "main", platform.WindowChrome{Title: "x"}); err == nil {
		t.Fatal("expected size validation")
	}
	missing := &desktop.WindowService{Gateway: allowAll{}, Host: okHost}
	if err := missing.Apply(ctx, caller, "main", chrome); err == nil {
		t.Fatal("expected missing adapter")
	}
	if _, err := missing.Read(ctx, caller, "main"); err == nil {
		t.Fatal("expected missing reader")
	}
	reader := &desktop.WindowService{
		Gateway: allowAll{},
		Host:    okHost,
		OnRead: func(_ context.Context, window domain.WindowID) (platform.WindowChrome, error) {
			if window != "main" {
				t.Fatalf("read window=%s", window)
			}
			return chrome, nil
		},
	}
	gotRead, err := reader.Read(ctx, caller, "main")
	if err != nil || gotRead.Title != "Vitra" || gotRead.Width != 800 {
		t.Fatalf("read: %+v err=%v", gotRead, err)
	}
	if err := missing.Focus(ctx, caller, "main"); err == nil {
		t.Fatal("expected missing focus adapter")
	}
	var focused domain.WindowID
	focuser := &desktop.WindowService{
		Gateway: allowAll{},
		Host:    okHost,
		OnFocus: func(_ context.Context, window domain.WindowID) error {
			focused = window
			return nil
		},
	}
	if err := focuser.Focus(ctx, caller, "main"); err != nil || focused != "main" {
		t.Fatalf("focus: %s err=%v", focused, err)
	}
	if err := focuser.Focus(ctx, caller, ""); err == nil {
		t.Fatal("expected empty window validation")
	}

	createHost := withFeatures(platform.OSLinux, platform.FeatureWindowCreate)
	var created desktop.WindowCreateOptions
	lifecycle := &desktop.WindowService{
		Gateway: allowAll{},
		Host:    createHost,
		OnCreate: func(_ context.Context, opts desktop.WindowCreateOptions) error {
			created = opts
			return nil
		},
		OnClose: func(_ context.Context, window domain.WindowID) error {
			if window != "aux" {
				t.Fatalf("close window=%s", window)
			}
			return nil
		},
	}
	id, err := lifecycle.Create(ctx, caller, desktop.WindowCreateOptions{ID: "aux", Title: "Aux", Width: 400, Height: 300})
	if err != nil || id != "aux" || created.Title != "Aux" {
		t.Fatalf("create: id=%s opts=%+v err=%v", id, created, err)
	}
	if err := lifecycle.Close(ctx, caller, "aux"); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Create(ctx, caller, desktop.WindowCreateOptions{}); err == nil {
		t.Fatal("expected empty id validation")
	}
	bareLife := &desktop.WindowService{Gateway: allowAll{}, Host: createHost}
	if _, err := bareLife.Create(ctx, caller, desktop.WindowCreateOptions{ID: "x"}); err == nil {
		t.Fatal("expected missing create adapter")
	}
	opts, err := desktop.ParseWindowCreateOptions(map[string]any{
		"id": "aux", "title": "T", "path": "/x", "width": 320.0, "height": 240.0,
	})
	if err != nil || opts.ID != "aux" || opts.Width != 320 || opts.Height != 240 {
		t.Fatalf("parse create: %+v err=%v", opts, err)
	}
	wid, err := desktop.ParseWindowID("aux")
	if err != nil || wid != "aux" {
		t.Fatalf("parse id: %v %v", wid, err)
	}
	winID, parsedChrome, err := desktop.ParseWindowChromeApply(map[string]any{
		"id": "main", "title": "Hi", "width": 640.0, "height": 480.0, "alwaysOnTop": true,
	})
	if err != nil || winID != "main" || parsedChrome.Title != "Hi" || parsedChrome.Width != 640 || !parsedChrome.AlwaysOnTop {
		t.Fatalf("parse chrome: %s %+v err=%v", winID, parsedChrome, err)
	}
}

func TestBrowserService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)
	ctx := context.Background()

	denied := &desktop.BrowserService{Gateway: denyAll{}, Host: host}
	err := denied.OpenURL(ctx, caller, "https://example.com")
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	bare := &desktop.BrowserService{Gateway: allowAll{}, Host: host}
	err = bare.OpenURL(ctx, caller, "https://example.com")
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureOpenURL {
		t.Fatalf("expected unsupported browser.open, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeatureOpenURL)
	var got string
	ok := &desktop.BrowserService{
		Gateway: allowAll{},
		Host:    okHost,
		OnOpen: func(_ context.Context, rawURL string) error {
			got = rawURL
			return nil
		},
	}
	if err := ok.OpenURL(ctx, caller, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com" {
		t.Fatalf("got %q", got)
	}
	if err := ok.OpenURL(ctx, caller, "  "); err == nil {
		t.Fatal("expected empty url validation")
	}
	missing := &desktop.BrowserService{Gateway: allowAll{}, Host: okHost}
	if err := missing.OpenURL(ctx, caller, "https://example.com"); err == nil {
		t.Fatal("expected missing adapter")
	}
}

func TestOsService_GrantAndDefault(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	ctx := context.Background()

	denied := &desktop.OsService{Gateway: denyAll{}}
	_, err := denied.Info(ctx, caller)
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	ok := &desktop.OsService{Gateway: allowAll{}}
	info, err := ok.Info(ctx, caller)
	if err != nil {
		t.Fatal(err)
	}
	if info.OS == "" || info.Arch == "" || info.Family == "" {
		t.Fatalf("incomplete default info: %+v", info)
	}

	var called bool
	custom := &desktop.OsService{
		Gateway: allowAll{},
		OnInfo: func(_ context.Context) (desktop.OsInfo, error) {
			called = true
			return desktop.OsInfo{OS: "test", Arch: "cpu", Family: "unix", Version: "1"}, nil
		},
	}
	got, err := custom.Info(ctx, caller)
	if err != nil || !called || got.Version != "1" {
		t.Fatalf("custom OnInfo: got=%+v err=%v called=%v", got, err, called)
	}
}

func TestNotificationService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)
	ctx := context.Background()

	denied := &desktop.NotificationService{Gateway: denyAll{}, Host: host}
	err := denied.Show(ctx, caller, "t", "b")
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	bare := &desktop.NotificationService{Gateway: allowAll{}, Host: host}
	err = bare.Show(ctx, caller, "t", "b")
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureNotificationShow {
		t.Fatalf("expected unsupported notification, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeatureNotificationShow)
	var gotTitle, gotBody string
	ok := &desktop.NotificationService{
		Gateway: allowAll{},
		Host:    okHost,
		OnShow: func(_ context.Context, title, body string) error {
			gotTitle, gotBody = title, body
			return nil
		},
	}
	if err := ok.Show(ctx, caller, "Hello", "World"); err != nil {
		t.Fatal(err)
	}
	if gotTitle != "Hello" || gotBody != "World" {
		t.Fatalf("got %q %q", gotTitle, gotBody)
	}
	if err := ok.Show(ctx, caller, "  ", "  "); err == nil {
		t.Fatal("expected empty validation")
	}
	missing := &desktop.NotificationService{Gateway: allowAll{}, Host: okHost}
	if err := missing.Show(ctx, caller, "t", "b"); err == nil {
		t.Fatal("expected missing adapter")
	}
}

func TestPathService_GrantFeatureAndHook(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := null.New(platform.OSLinux)
	ctx := context.Background()

	denied := &desktop.PathService{Gateway: denyAll{}, Host: host}
	err := denied.Open(ctx, caller, "/tmp/x")
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	bare := &desktop.PathService{Gateway: allowAll{}, Host: host}
	err = bare.Open(ctx, caller, "/tmp/x")
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeaturePathOpen {
		t.Fatalf("expected unsupported path.open, got %v", err)
	}

	okHost := withFeatures(platform.OSLinux, platform.FeaturePathOpen)
	var got string
	ok := &desktop.PathService{
		Gateway: allowAll{},
		Host:    okHost,
		OnOpen: func(_ context.Context, path string) error {
			got = path
			return nil
		},
	}
	if err := ok.Open(ctx, caller, "/tmp/vitra-path"); err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/vitra-path" {
		t.Fatalf("got %q", got)
	}
	for _, bad := range []string{"", "relative", "https://example.com", "file:///etc/passwd"} {
		if err := ok.Open(ctx, caller, bad); err == nil {
			t.Fatalf("expected validation for %q", bad)
		}
	}
	missing := &desktop.PathService{Gateway: allowAll{}, Host: okHost}
	if err := missing.Open(ctx, caller, "/tmp/x"); err == nil {
		t.Fatal("expected missing adapter")
	}
}
