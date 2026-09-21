package packaging_test

import (
	"strings"
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

func TestSpec_RejectsRawKeyMaterial(t *testing.T) {
	base := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Example",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Sign:    true,
	}
	for _, ref := range []string{
		"-----BEGIN PRIVATE KEY-----\nMII…",
		"BEGIN RSA PRIVATE KEY",
		"not-a-prefix",
		"env:",
		"file:",
	} {
		s := base
		s.SigningIdentityRef = ref
		err := s.Validate()
		if err == nil {
			t.Fatalf("expected reject for %q", ref)
		}
		if !strings.Contains(err.Error(), "SigningIdentityRef") {
			t.Fatalf("unexpected error for %q: %v", ref, err)
		}
	}
}

func TestSpec_AcceptsSigningRefPrefixes(t *testing.T) {
	base := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Example",
		Targets: []packaging.Target{packaging.TargetWindowsMSI},
		Sign:    true,
	}
	for _, ref := range []string{
		"env:CODESIGN_ID",
		"keychain:Developer ID Application: Example",
		"file:/run/secrets/codesign.p12",
		"secret:ci/codesign",
	} {
		s := base
		s.SigningIdentityRef = ref
		if err := s.Validate(); err != nil {
			t.Fatalf("%q: %v", ref, err)
		}
	}
}
