package darwin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestRegisterURLScheme_WritesHelperApp(t *testing.T) {
	tmp := t.TempDir()
	prevHome, prevLS := supportHome, lsregisterRunner
	t.Cleanup(func() {
		supportHome = prevHome
		lsregisterRunner = prevLS
	})
	supportHome = func() (string, error) { return tmp, nil }
	var registered string
	lsregisterRunner = func(appPath string) { registered = appPath }

	bin := filepath.Join(tmp, "appbin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterURLScheme("vitra", "com.vitra.test", bin); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(tmp, "com.vitra.test-url.app", "Contents", "Info.plist")
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"<string>vitra</string>",
		"<key>CFBundleURLSchemes</key>",
		"<string>com.vitra.test</string>",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("plist missing %q:\n%s", want, s)
		}
	}
	launcher := filepath.Join(tmp, "com.vitra.test-url.app", "Contents", "MacOS", "launcher")
	script, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), bin) {
		t.Fatalf("launcher missing exec path: %s", script)
	}
	if registered != filepath.Join(tmp, "com.vitra.test-url.app") {
		t.Fatalf("lsregister path=%q", registered)
	}
	if !h.Features()[platform.FeatureDeepLink].Available {
		t.Fatal("expected deeplink available")
	}
	if err := h.UnregisterURLScheme("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "com.vitra.test-url.app")); !os.IsNotExist(err) {
		t.Fatalf("expected bundle removed, err=%v", err)
	}
}

func TestRegisterURLScheme_RejectsBadScheme(t *testing.T) {
	h := New()
	if err := h.RegisterURLScheme("Bad Scheme", "com.x", "/bin/true"); err == nil {
		t.Fatal("expected rejection")
	}
}
