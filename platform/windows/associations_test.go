package windows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestRegisterFileAssociations_WritesReg(t *testing.T) {
	tmp := t.TempDir()
	prevHome, prevImp := supportHome, registryImportRunner
	t.Cleanup(func() {
		supportHome = prevHome
		registryImportRunner = prevImp
	})
	supportHome = func() (string, error) { return tmp, nil }
	registryImportRunner = func(string) {}

	bin := filepath.Join(tmp, "appbin.exe")
	if err := os.WriteFile(bin, []byte("MZ"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterFileAssociations("com.vitra.test", bin, "Vitra Test", []string{"text/plain", "application/json"}); err != nil {
		t.Fatal(err)
	}
	regPath := filepath.Join(tmp, "com.vitra.test-files.reg")
	body, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		`com.vitra.test.file`,
		`text/plain`,
		`application/json`,
		`Vitra Test`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("reg missing %q:\n%s", want, s)
		}
	}
	if !h.Features()[platform.FeatureFileAssociation].Available {
		t.Fatal("expected file_association available")
	}
	if err := h.UnregisterFileAssociations("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(regPath); !os.IsNotExist(err) {
		t.Fatalf("expected reg removed, err=%v", err)
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
