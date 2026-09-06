package application_test

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/application"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/inmemory"
)

func TestCloseWindow_ReleasesSubscriptions(t *testing.T) {
	// Reliability invariant 2 / security invariant 15.
	windows := inmemory.NewWindowRepo()
	subs := inmemory.NewSubscriptionRepo()
	open := &application.OpenWindowUseCase{Windows: windows}
	subscribe := &application.SubscribeEventUseCase{Windows: windows, Subscriptions: subs}
	closeUC := &application.CloseWindowUseCase{Windows: windows, Subscriptions: subs}

	if _, err := open.Execute("main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	sub, err := subscribe.Execute("sub-1", "app.tick", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := closeUC.Execute("main"); err != nil {
		t.Fatal(err)
	}
	if !sub.IsClosed() {
		t.Fatal("subscription must be closed")
	}
	if _, err := subs.Get("sub-1"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatalf("subscription must be deleted: %v", err)
	}
}

func TestNavigateWithPolicy_DeniesUntrusted(t *testing.T) {
	windows := inmemory.NewWindowRepo()
	policy, _ := domain.NewNavigationPolicy(nil, false)
	open := &application.OpenWindowUseCase{Windows: windows}
	nav := &application.NavigateWithPolicyUseCase{Windows: windows, Policy: policy}
	if _, err := open.Execute("main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	evil, _ := domain.NewOrigin("https://evil.example")
	err := nav.Execute("main", evil)
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialOriginMismatch {
		t.Fatalf("expected origin denial, got %v", err)
	}
}
