package windows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestRegisterURLScheme_WritesReg(t *testing.T) {
	tmp := t.TempDir()
	prevHome, prevImp := supportHome, registryImportRunner
	t.Cleanup(func() {
		supportHome = prevHome
		registryImportRunner = prevImp
	})
	supportHome = func() (string, error) { return tmp, nil }
	var imported string
	registryImportRunner = func(regPath string) { imported = regPath }

	bin := filepath.Join(tmp, "appbin.exe")
	if err := os.WriteFile(bin, []byte("MZ"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterURLScheme("vitra", "com.vitra.test", bin); err != nil {
		t.Fatal(err)
	}
	regPath := filepath.Join(tmp, "com.vitra.test-url.reg")
	body, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		`Software\Classes\vitra`,
		`"URL Protocol"=""`,
		bin,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("reg missing %q:\n%s", want, s)
		}
	}
	if imported != regPath {
		t.Fatalf("import path=%q", imported)
	}
	if !h.Features()[platform.FeatureDeepLink].Available {
		t.Fatal("expected deeplink available")
	}
	if err := h.UnregisterURLScheme("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(regPath); !os.IsNotExist(err) {
		t.Fatalf("expected reg removed, err=%v", err)
	}
}

func TestRegisterURLScheme_RejectsBadScheme(t *testing.T) {
	h := New()
	if err := h.RegisterURLScheme("Bad Scheme", "com.x", "/bin/true"); err == nil {
		t.Fatal("expected rejection")
	}
}
