package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestRegisterFileAssociations_WritesDesktop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	bin := filepath.Join(tmp, "appbin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := New()
	if err := h.RegisterFileAssociations("com.vitra.test", bin, "Vitra Test", []string{"text/plain", "application/json"}); err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(tmp, "applications", "com.vitra.test-files.desktop")
	body, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"MimeType=text/plain;application/json;",
		"Exec=" + bin + " %f",
		"Name=Vitra Test",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("desktop missing %q:\n%s", want, s)
		}
	}
	fs := h.Features()
	if !fs[platform.FeatureFileAssociation].Available {
		t.Fatal("linux host should report file_association available")
	}
	if err := h.UnregisterFileAssociations("com.vitra.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatalf("expected desktop removed, err=%v", err)
	}
}

func TestRegisterFileAssociations_RejectsBadMIME(t *testing.T) {
	h := New()
	if err := h.RegisterFileAssociations("com.x", "/bin/true", "X", []string{"not a mime"}); err == nil {
		t.Fatal("expected invalid mime error")
	}
	if err := h.RegisterFileAssociations("com.x", "/bin/true", "X", nil); err == nil {
		t.Fatal("expected missing mime error")
	}
}
