package domain_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestSubscription_OwnedByWindow(t *testing.T) {
	sub, err := domain.NewSubscription("sub-1", "app.ready", "main")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Owner() != "main" || sub.IsClosed() {
		t.Fatal("bad subscription")
	}
	sub.Close()
	if !sub.IsClosed() {
		t.Fatal("expected closed")
	}
}

func TestNavigationPolicy_UntrustedDeniedByDefault(t *testing.T) {
	// Security invariant 12.
	p, err := domain.NewNavigationPolicy(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if d := p.AllowInWebView(domain.OriginPackagedLocal); !d.Allowed {
		t.Fatalf("packaged: %+v", d)
	}
	trusted, _ := domain.NewOrigin("https://app.example")
	p2, _ := domain.NewNavigationPolicy([]domain.Origin{trusted}, false)
	if d := p2.AllowInWebView(trusted); !d.Allowed {
		t.Fatalf("trusted: %+v", d)
	}
	evil, _ := domain.NewOrigin("https://evil.example")
	if d := p2.AllowInWebView(evil); d.Allowed || d.Code != domain.DenialOriginMismatch {
		t.Fatalf("untrusted must be denied: %+v", d)
	}
}

func TestDeepLinkPattern(t *testing.T) {
	pat := domain.DeepLinkPattern{Scheme: "vitra", Host: "open", Path: "/project"}
	if !pat.Match("vitra://open/project/123") {
		t.Fatal("expected match")
	}
	if pat.Match("https://open/project/123") {
		t.Fatal("wrong scheme")
	}
}
