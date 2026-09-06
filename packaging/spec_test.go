package packaging_test

import (
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestSpec_RejectsInlineSigningSecretPattern(t *testing.T) {
	// Security invariant 10: signing secrets must not reside in project config.
	// Spec only accepts identity *references*, never raw key material fields.
	s := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Example",
		Targets: []packaging.Target{packaging.TargetLinuxAppImage},
		Sign:    true,
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when Sign without SigningIdentityRef")
	}
	s.SigningIdentityRef = "env:APPLE_IDENTITY"
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
