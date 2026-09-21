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
	ok := &desktop.MenuService{
		Gateway: allowAll{},
		Host:    okHost,
		OnSet: func(ctx context.Context, items []desktop.MenuItem) error {
			called = true
			return nil
		},
	}
	if err := ok.SetMenu(context.Background(), caller, []desktop.MenuItem{{ID: "quit", Label: "Quit"}}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected OnSet")
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

	if err := (&desktop.TrayService{
		Gateway: allowAll{}, Host: host,
		OnSet: func(context.Context, string, []desktop.MenuItem) error { return nil },
	}).SetTray(ctx, caller, "Vitra", nil); err != nil {
		t.Fatal(err)
	}

	dlg := &desktop.DialogService{
		Gateway: allowAll{}, Host: host,
		OnOpen: func(context.Context) ([]string, error) { return []string{"/tmp/a"}, nil },
		OnSave: func(context.Context) (string, error) { return "/tmp/b", nil },
	}
	paths, err := dlg.OpenFile(ctx, caller)
	if err != nil || len(paths) != 1 {
		t.Fatalf("open: %v %v", paths, err)
	}
	save, err := dlg.SaveFile(ctx, caller)
	if err != nil || save != "/tmp/b" {
		t.Fatalf("save: %v %v", save, err)
	}
	bare := &desktop.DialogService{Gateway: allowAll{}, Host: host}
	if _, err := bare.OpenFile(ctx, caller); err == nil {
		t.Fatal("expected missing open adapter")
	}
	if _, err := bare.SaveFile(ctx, caller); err == nil {
		t.Fatal("expected missing save adapter")
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

	sc := &desktop.ShortcutService{
		Gateway: allowAll{}, Host: host,
		OnRegister: func(context.Context, string) error { return nil },
	}
	if err := sc.Register(ctx, caller, "Ctrl+Shift+P"); err != nil {
		t.Fatal(err)
	}
	if err := sc.Register(ctx, caller, ""); err == nil {
		t.Fatal("expected empty accelerator validation")
	}

	si := &desktop.SingleInstanceService{
		Gateway: allowAll{}, Host: host,
		OnLock: func(context.Context) (bool, error) { return true, nil },
	}
	ok, err := si.Acquire(ctx, caller)
	if err != nil || !ok {
		t.Fatalf("acquire: %v %v", ok, err)
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
