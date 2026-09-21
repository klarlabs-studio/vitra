package darwin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestRegisterFileAssociations_WritesHelperApp(t *testing.T) {
	tmp := t.TempDir()
	prevHome, prevLS := supportHome, lsregisterRunner
	t.Cleanup(func() {
		supportHome = prevHome
		lsregisterRunner = prevLS
	})
	supportHome = func() (string, error) { return tmp, nil }
	lsregisterRunner = func(string) {}

	bin := filepath.Join(tmp, "appbin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterFileAssociations("com.vitra.test", bin, "Vitra Test", []string{"text/plain", "application/json"}); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(tmp, "com.vitra.test-files.app", "Contents", "Info.plist")
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"<string>text/plain</string>",
		"<string>application/json</string>",
		"<key>CFBundleDocumentTypes</key>",
		"<string>Vitra Test</string>",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("plist missing %q:\n%s", want, s)
		}
	}
	if !h.Features()[platform.FeatureFileAssociation].Available {
		t.Fatal("expected file_association available")
	}
	if err := h.UnregisterFileAssociations("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "com.vitra.test-files.app")); !os.IsNotExist(err) {
		t.Fatalf("expected bundle removed, err=%v", err)
	}
}

func TestRegisterFileAssociations_RejectsBadMIME(t *testing.T) {
	h := New()
	if err := h.RegisterFileAssociations("com.x", "/bin/true", "X", []string{"not a mime"}); err == nil {
		t.Fatal("expected rejection")
	}
	if err := h.RegisterFileAssociations("com.x", "/bin/true", "X", nil); err == nil {
		t.Fatal("expected rejection")
	}
}
