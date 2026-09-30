package domain_test

import (
	"fmt"

	"go.klarlabs.de/vitra/domain"
)

// Deny wins over allow, deny ignores letter case, and paths that could escape
// the scope are refused before any pattern is consulted.
func ExamplePathScope_Matches() {
	scope := domain.PathScope{
		Allow: []string{"/notes/**/*.md"},
		Deny:  []string{"/notes/.private/**"},
	}
	for _, p := range []string{
		"/notes/todo.md",
		"/notes/projects/plan.md",
		"/notes/run.sh",
		"/notes/.private/keys.md",
		"/notes/.PRIVATE/keys.md",
		"/notes/../etc/passwd",
	} {
		ok, code, _ := scope.Matches(p)
		if ok {
			fmt.Println(p, "allowed")
		} else {
			fmt.Println(p, code)
		}
	}
	// Output:
	// /notes/todo.md allowed
	// /notes/projects/plan.md allowed
	// /notes/run.sh path_out_of_scope
	// /notes/.private/keys.md path_denied
	// /notes/.PRIVATE/keys.md path_denied
	// /notes/../etc/passwd path_denied
}

// A grant names windows, origins, and permissions. There is no wildcard, and
// malformed patterns are rejected when the grant is built.
func ExampleNewCapabilityGrant() {
	_, err := domain.NewCapabilityGrant("editor", "edit notes",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.write", PathScope: &domain.PathScope{
			Allow: []string{"notes/**"}, // relative: rejected
		}}})
	fmt.Println(err != nil)

	grant, err := domain.NewCapabilityGrant("editor", "edit notes",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.write", PathScope: &domain.PathScope{
			Allow: []string{"/home/me/Notes/**/*.md"},
		}}})
	if err != nil {
		panic(err)
	}
	fmt.Println(grant.Name())
	// Output:
	// true
	// editor
}
