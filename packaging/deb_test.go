package packaging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestBuildAppDir_Layout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "App.AppDir")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxAppImage},
	}
	art, err := packaging.BuildAppDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxAppImage {
		t.Fatalf("%+v", art)
	}
	for _, p := range []string{
		filepath.Join(out, "AppRun"),
		filepath.Join(out, "usr", "bin", "Vitra-Demo"),
		filepath.Join(out, "Vitra-Demo.desktop"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	body, _ := os.ReadFile(filepath.Join(out, "AppRun"))
	if !strings.Contains(string(body), "usr/bin/Vitra-Demo") {
		t.Fatalf("AppRun=%s", body)
	}
}

func TestBuildDeb_ArArchive(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	payload := []byte("#!/bin/sh\necho vitra-deb\n")
	if err := os.WriteFile(bin, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64",
	}
	art, err := packaging.BuildDeb(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxDeb || art.Path != out {
		t.Fatalf("%+v", art)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "!<arch>\n") {
		t.Fatal("not an ar archive")
	}
	if !strings.Contains(string(raw), "debian-binary") || !strings.Contains(string(raw), "control.tar.gz") {
		t.Fatal("missing ar members")
	}
	if _, err := exec.LookPath("dpkg-deb"); err == nil {
		cmd := exec.Command("dpkg-deb", "--info", out)
		outb, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("dpkg-deb: %v (%s)", err, outb)
		}
		if !strings.Contains(string(outb), "Package: com-vitra-demo") {
			t.Fatalf("info=%s", outb)
		}
	}
}
