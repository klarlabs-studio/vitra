package domain_test

import (
	"slices"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

// A deny written with one spelling of a directory must also cover its real
// spelling (macOS /var is /private/var).
func TestPathScope_WithDenyAliases(t *testing.T) {
	resolve := func(dir string) (string, bool) {
		switch dir {
		case "/var/app/secret":
			return "/private/var/app/secret", true
		case "/var/app":
			return "/private/var/app", true
		}
		return "", false
	}
	scope := domain.PathScope{
		Allow: []string{"/var/app/**"},
		Deny:  []string{"/var/app/secret/**", "/var/app/**/*.pem", "/home/me/x/**"},
	}
	got := scope.WithDenyAliases(resolve)
	want := []string{"/var/app/secret/**", "/var/app/**/*.pem", "/home/me/x/**", "/private/var/app/secret/**", "/private/var/app/**/*.pem"}
	if !slices.Equal(got.Deny, want) {
		t.Fatalf("deny = %v, want %v", got.Deny, want)
	}
	if !slices.Equal(got.Allow, scope.Allow) {
		t.Fatalf("allow changed: %v", got.Allow)
	}
	if len(scope.Deny) != 3 {
		t.Fatal("WithDenyAliases modified the receiver")
	}
	// Idempotent: resolving again adds nothing new.
	if again := got.WithDenyAliases(resolve); !slices.Equal(again.Deny, want) {
		t.Fatalf("second pass deny = %v", again.Deny)
	}
}

func TestCapabilityGrant_WithDenyAliases(t *testing.T) {
	g, err := domain.NewCapabilityGrant("g", "g", []domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "fs.read", PathScope: &domain.PathScope{Allow: []string{"/private/var/app/**"}, Deny: []string{"/var/app/secret/**"}}},
			{Name: "notes.list"},
		})
	if err != nil {
		t.Fatal(err)
	}
	g2, err := g.WithDenyAliases(func(dir string) (string, bool) {
		if dir == "/var/app/secret" {
			return "/private/var/app/secret", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	if d := g.Authorize(caller, "fs.read", "/private/var/app/secret/k"); !d.Allowed {
		t.Fatal("precondition: original grant should miss the alias")
	}
	if d := g2.Authorize(caller, "fs.read", "/private/var/app/secret/k"); d.Allowed || d.Code != domain.DenialPathDenied {
		t.Fatalf("aliased deny not applied: %+v", d)
	}
	if d := g2.Authorize(caller, "notes.list", ""); !d.Allowed {
		t.Fatalf("unscoped permission lost: %+v", d)
	}
}
