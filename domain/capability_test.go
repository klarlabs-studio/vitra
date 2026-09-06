package domain_test

import (
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestPathScope_DenyWinsAndTraversalRejected(t *testing.T) {
	scope := domain.PathScope{
		Allow: []string{"/project/**"},
		Deny:  []string{"/project/.secrets/**"},
	}

	ok, _, _ := scope.Matches("/project/src/main.go")
	if !ok {
		t.Fatal("expected allow")
	}

	ok, code, _ := scope.Matches("/project/.secrets/token")
	if ok || code != domain.DenialPathDenied {
		t.Fatalf("deny: ok=%v code=%s", ok, code)
	}

	ok, code, reason := scope.Matches("/project/../etc/passwd")
	if ok || code != domain.DenialPathDenied {
		t.Fatalf("traversal: ok=%v code=%s reason=%s", ok, code, reason)
	}

	ok, code, _ = scope.Matches("/other/file")
	if ok || code != domain.DenialPathOutOfScope {
		t.Fatalf("out of scope: ok=%v code=%s", ok, code)
	}
}

func TestCapabilityGrant_RequiresExplicitWindowsOriginsPermissions(t *testing.T) {
	name, _ := domain.NewGrantName("project-files")
	perm, _ := domain.NewPermissionName("fs.read")
	win, _ := domain.NewWindowID("main")

	_, err := domain.NewCapabilityGrant(name, "files", nil, []domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{Name: perm}})
	if !errors.Is(err, &domain.ErrValidation{}) {
		t.Fatalf("expected validation for empty windows, got %v", err)
	}
	_, err = domain.NewCapabilityGrant(name, "files", []domain.WindowID{win}, nil, []domain.PermissionSpec{{Name: perm}})
	if !errors.Is(err, &domain.ErrValidation{}) {
		t.Fatalf("expected validation for empty origins, got %v", err)
	}
	_, err = domain.NewCapabilityGrant(name, "files", []domain.WindowID{win}, []domain.Origin{domain.OriginPackagedLocal}, nil)
	if !errors.Is(err, &domain.ErrValidation{}) {
		t.Fatalf("expected validation for empty permissions, got %v", err)
	}
}

func TestCapabilityGrant_AuthorizeWindowOriginPermission(t *testing.T) {
	grant := mustGrant(t)
	caller, err := domain.NewCaller("main", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}

	d := grant.Authorize(caller, "fs.read", "/project/a.go")
	if !d.Allowed || d.Grant != "project-files" {
		t.Fatalf("expected allow, got %+v", d)
	}

	d = grant.Authorize(caller, "fs.write", "/project/a.go")
	if d.Allowed || d.Code != domain.DenialPermissionAbsent {
		t.Fatalf("unexpected: %+v", d)
	}

	other, _ := domain.NewCaller("other", domain.OriginPackagedLocal)
	d = grant.Authorize(other, "fs.read", "/project/a.go")
	if d.Allowed || d.Code != domain.DenialWindowMismatch {
		t.Fatalf("unexpected: %+v", d)
	}

	remote, _ := domain.NewCaller("main", "https://evil.example")
	d = grant.Authorize(remote, "fs.read", "/project/a.go")
	if d.Allowed || d.Code != domain.DenialOriginMismatch {
		t.Fatalf("unexpected: %+v", d)
	}
}

func mustGrant(t *testing.T) *domain.CapabilityGrant {
	t.Helper()
	name, _ := domain.NewGrantName("project-files")
	perm, _ := domain.NewPermissionName("fs.read")
	win, _ := domain.NewWindowID("main")
	scope := &domain.PathScope{
		Allow: []string{"/project/**"},
		Deny:  []string{"/project/.secrets/**"},
	}
	g, err := domain.NewCapabilityGrant(
		name,
		"project file access",
		[]domain.WindowID{win},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: perm, PathScope: scope}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
