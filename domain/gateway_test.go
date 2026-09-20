package domain_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestGateway_NewWindowHasNoPrivileges(t *testing.T) {
	// Security invariant 1: a newly created WebView has no privileged native
	// API access unless a capability grants it.
	gw := domain.NewCapabilityGateway() // no grants
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	d := gw.Authorize(caller, "fs.read", "/project/a.go")
	if d.Allowed || d.Code != domain.DenialNoGrant {
		t.Fatalf("expected no_grant denial, got %+v", d)
	}
}

func TestGateway_NavigationDropsAuthority(t *testing.T) {
	// Security invariant 2: navigation to a new origin cannot retain authority
	// granted to another origin.
	grant := mustGrant(t)
	gw := domain.NewCapabilityGateway(grant)

	win, err := domain.NewWindow("main", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}
	caller, err := win.Caller()
	if err != nil {
		t.Fatal(err)
	}
	if d := gw.Authorize(caller, "fs.read", "/project/a.go"); !d.Allowed {
		t.Fatalf("expected allow before navigate: %+v", d)
	}

	if err := win.Navigate("https://untrusted.example"); err != nil {
		t.Fatal(err)
	}
	caller, err = win.Caller()
	if err != nil {
		t.Fatal(err)
	}
	d := gw.Authorize(caller, "fs.read", "/project/a.go")
	if d.Allowed || d.Code != domain.DenialOriginMismatch {
		t.Fatalf("expected origin mismatch after navigate, got %+v", d)
	}
}

func TestGateway_RemoteContentDeniedByDefault(t *testing.T) {
	// Security invariant 12: remote/untrusted content has no native authority
	// by default.
	grant := mustGrant(t)
	gw := domain.NewCapabilityGateway(grant)
	caller, _ := domain.NewCaller("main", "https://cdn.example")
	d := gw.Authorize(caller, "fs.read", "/project/a.go")
	if d.Allowed {
		t.Fatalf("remote must be denied: %+v", d)
	}
}

func TestGateway_DenialDeterministicAndInspectable(t *testing.T) {
	// Security invariant 13.
	grant := mustGrant(t)
	gw := domain.NewCapabilityGateway(grant)
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)

	d1 := gw.Authorize(caller, "shell.exec", "")
	d2 := gw.Authorize(caller, "shell.exec", "")
	if d1 != d2 {
		t.Fatalf("denials not deterministic: %+v vs %+v", d1, d2)
	}
	if d1.Code != domain.DenialNoGrant || d1.Reason == "" {
		t.Fatalf("expected inspectable denial: %+v", d1)
	}
}

func TestGateway_InspectEffectiveSurface(t *testing.T) {
	grant := mustGrant(t)
	gw := domain.NewCapabilityGateway(grant)
	surface := gw.Inspect("main", domain.OriginPackagedLocal)
	if len(surface.GrantNames) != 1 || surface.GrantNames[0] != "project-files" {
		t.Fatalf("grants=%v", surface.GrantNames)
	}
	if len(surface.Permissions) != 1 || surface.Permissions[0].Name != "fs.read" {
		t.Fatalf("perms=%v", surface.Permissions)
	}
	empty := gw.Inspect("main", "https://evil.example")
	if len(empty.GrantNames) != 0 || len(empty.Permissions) != 0 {
		t.Fatalf("expected empty surface for other origin: %+v", empty)
	}
}
