package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterURLScheme_WritesDesktop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	bin := filepath.Join(tmp, "appbin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterURLScheme("vitra", "com.vitra.test", bin); err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(tmp, "applications", "com.vitra.test-url.desktop")
	body, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"MimeType=x-scheme-handler/vitra;",
		"Exec=" + bin + " %u",
		"Name=com.vitra.test",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("desktop missing %q:\n%s", want, s)
		}
	}
	if err := h.UnregisterURLScheme("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatalf("expected desktop removed, err=%v", err)
	}
}

func TestRegisterURLScheme_RejectsBadScheme(t *testing.T) {
	h := New()
	if err := h.RegisterURLScheme("Bad Scheme", "com.x", "/bin/true"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
}
