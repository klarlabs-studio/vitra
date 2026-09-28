package packaging_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestStageLinux_LayoutAndDigest(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	payload := []byte("#!/bin/sh\necho vitra\n")
	if err := os.WriteFile(bin, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := packaging.Spec{
		AppID:   "com.vitra.demo",
		Version: "0.3.0",
		Name:    "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDir},
		Arch:    packaging.DefaultArch(),
	}
	art, err := packaging.StageLinux(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxDir || art.Path != out {
		t.Fatalf("artifact=%+v", art)
	}
	sum := sha256.Sum256(payload)
	if art.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha=%s", art.SHA256)
	}
	staged := filepath.Join(out, "bin", "Vitra-Demo")
	if _, err := os.Stat(staged); err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(out, "com.vitra.demo.desktop")
	body, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Exec="+staged) {
		t.Fatalf("desktop=%s", body)
	}
	if !strings.Contains(string(body), "StartupWMClass=Vitra-Demo") {
		t.Fatalf("desktop missing StartupWMClass: %s", body)
	}
	if !strings.Contains(string(body), "X-GNOME-UsesNotifications=true") {
		t.Fatalf("desktop missing X-GNOME-UsesNotifications: %s", body)
	}
	if !strings.Contains(string(body), "StartupNotify=true") {
		t.Fatalf("desktop missing StartupNotify: %s", body)
	}
	if !strings.Contains(string(body), "SingleMainWindow=true") {
		t.Fatalf("desktop missing SingleMainWindow: %s", body)
	}
	if !strings.Contains(string(body), "Terminal=false") {
		t.Fatalf("desktop missing Terminal=false: %s", body)
	}
	if !strings.Contains(string(body), "Comment="+packaging.DefaultDescription) {
		t.Fatalf("desktop missing Comment: %s", body)
	}
	meta := filepath.Join(out, "usr", "share", "metainfo", "com.vitra.demo.metainfo.xml")
	if _, err := os.Stat(meta); err != nil {
		t.Fatal(err)
	}
}

func TestStageLinux_RejectsInlineSigningSecretPattern(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.x", Version: "1", Name: "X",
		Targets: []packaging.Target{packaging.TargetLinuxDir},
		Sign:    true,
		// empty ref must fail Validate
	}
	_, err := packaging.StageLinux(spec, "/bin/true", t.TempDir())
	if err == nil {
		t.Fatal("expected validate error")
	}
}

func TestStageLinux_RequiresLinuxDirTarget(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.x", Version: "1", Name: "X",
		Targets: []packaging.Target{packaging.TargetLinuxAppImage},
	}
	_, err := packaging.StageLinux(spec, "/bin/true", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "linux-dir") {
		t.Fatalf("got %v", err)
	}
}
