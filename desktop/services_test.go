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

func (denyAll) Authorize(caller domain.Caller, perm domain.PermissionName, _ string) domain.Decision {
	return domain.Decision{
		Permission: perm,
		Code:       domain.DenialNoGrant,
		Reason:     "denied in test",
	}
}

func TestMenuService_RequiresGrantAndFeature(t *testing.T) {
	host := null.New(platform.OSLinux)
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)

	denied := &desktop.MenuService{Gateway: denyAll{}, Host: host}
	err := denied.SetMenu(context.Background(), caller, nil)
	var d *domain.ErrDenied
	if !errors.As(err, &d) || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}

	// Null host does not support menu.bar → explicit unsupported after grant.
	svc := &desktop.MenuService{Gateway: allowAll{}, Host: host}
	err = svc.SetMenu(context.Background(), caller, []desktop.MenuItem{{ID: "quit", Label: "Quit"}})
	var un *platform.ErrUnsupported
	if !errors.As(err, &un) || un.Feature != platform.FeatureMenuBar {
		t.Fatalf("expected unsupported menu, got %v", err)
	}
}

func TestClipboard_DeniedWithoutGrant(t *testing.T) {
	host := null.New(platform.OSDarwin)
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	svc := &desktop.ClipboardService{Gateway: denyAll{}, Host: host}
	_, err := svc.Read(context.Background(), caller)
	var d *domain.ErrDenied
	if !errors.As(err, &d) {
		t.Fatalf("expected denial, got %v", err)
	}
}

func TestDeepLink_RequiresPatternMatch(t *testing.T) {
	// Inject a host that advertises deeplink support so pattern matching is exercised.
	h := &deeplinkHost{Host: null.New(platform.OSWindows)}
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	svc := &desktop.DeepLinkService{
		Gateway:  allowAll{},
		Host:     h,
		Patterns: []domain.DeepLinkPattern{{Scheme: "vitra", Host: "open"}},
	}
	ok, err := svc.Handle(caller, "vitra://open/x")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	_, err = svc.Handle(caller, "https://evil.example")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

type deeplinkHost struct{ *null.Host }

func (h *deeplinkHost) Features() platform.FeatureSet {
	fs := h.Host.Features()
	fs[platform.FeatureDeepLink] = platform.Support{
		Feature: platform.FeatureDeepLink, Available: true,
	}
	return fs
}
