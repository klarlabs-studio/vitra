package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Launch at login is an XDG autostart entry that runs the executable, quoted
// so that no path can inject arguments or field codes.
func TestLoginItem(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	exe := filepath.Join(t.TempDir(), `my "app" $HOME 100%`)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &Host{}
	const id = "com.example.Usage"
	if on, err := h.LoginItemEnabled(id); err != nil || on {
		t.Fatalf("before: %v, %v", on, err)
	}
	if err := h.SetLoginItem(id, exe, true); err != nil {
		t.Fatal(err)
	}
	if on, err := h.LoginItemEnabled(id); err != nil || !on {
		t.Fatalf("after enabling: %v, %v", on, err)
	}
	raw, err := os.ReadFile(filepath.Join(config, "autostart", id+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	wantExec := `Exec="` + strings.ReplaceAll(filepath.Dir(exe), `\`, `\\\\`) + `/my \\"app\\" \\$HOME 100%%"`
	if !strings.Contains(string(raw), wantExec+"\n") || !strings.Contains(string(raw), "Type=Application\n") {
		t.Fatalf("autostart entry:\n%s\nwant %s", raw, wantExec)
	}
	if err := h.SetLoginItem(id, "", false); err != nil {
		t.Fatal(err)
	}
	if on, _ := h.LoginItemEnabled(id); on {
		t.Fatal("still enabled after disabling")
	}
	// Disabling twice is fine; enabling needs an executable that exists.
	if err := h.SetLoginItem(id, "", false); err != nil {
		t.Fatal(err)
	}
	if err := h.SetLoginItem(id, filepath.Join(config, "missing"), true); err == nil {
		t.Fatal("enabled a missing executable")
	}
	if err := h.SetLoginItem("", exe, true); err == nil {
		t.Fatal("enabled without an app id")
	}
}

// An app id cannot place the entry outside the autostart folder.
func TestLoginItem_AppIDStaysInAutostart(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	path, err := autostartFile("../../evil/app")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(config, "autostart") {
		t.Fatalf("entry at %s", path)
	}
}
