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
		`@="\"` + strings.ReplaceAll(bin, `\`, `\\`) + `\" \"%1\""`,
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

// .reg string values escape backslashes and quotes; a raw quote ends the
// value early and a newline starts a new registry line.
func TestRegEscape(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Program Files\App\app.exe`:        `C:\\Program Files\\App\\app.exe`,
		`"C:\a b\app.exe" "%1"`:               `\"C:\\a b\\app.exe\" \"%1\"`,
		"My App\r\n[HKEY_CURRENT_USER\\Evil]": `My App[HKEY_CURRENT_USER\\Evil]`,
	} {
		if got := regEscape(in); got != want {
			t.Errorf("regEscape(%q) = %q, want %q", in, got, want)
		}
	}
}
