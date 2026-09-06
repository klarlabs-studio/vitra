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
