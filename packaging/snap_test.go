package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestBuildSnapDir_Layout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(tmp, "logo.png")
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "prime")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets:     []packaging.Target{packaging.TargetLinuxSnap},
		IconPath:    icon,
		Description: "Demo Snap",
		Homepage:    "https://example.com/demo",
		License:     "Apache-2.0",
	}
	art, err := packaging.BuildSnapDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxSnap || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	for _, p := range []string{
		filepath.Join(out, "usr", "bin", "Vitra-Demo"),
		filepath.Join(out, "usr", "share", "applications", "com.vitra.demo.desktop"),
		filepath.Join(out, "meta", "snap.yaml"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	yaml, err := os.ReadFile(filepath.Join(out, "meta", "snap.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(yaml)
	for _, want := range []string{
		"name: com-vitra-demo",
		`version: "0.3.0"`,
		"Demo Snap",
		"license: Apache-2.0",
		`website: "https://example.com/demo"`,
		"command: usr/bin/Vitra-Demo",
		"desktop: usr/share/applications/com.vitra.demo.desktop",
		"confinement: strict",
		"base: core22",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("snap.yaml missing %q:\n%s", want, text)
		}
	}
}

func TestBuildSnapDir_DefaultLicenseOmitsWebsite(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "prime")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxSnap},
	}
	if _, err := packaging.BuildSnapDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	yaml, err := os.ReadFile(filepath.Join(out, "meta", "snap.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(yaml)
	if !strings.Contains(text, "license: "+packaging.DefaultLicense) {
		t.Fatalf("default license missing:\n%s", text)
	}
	if strings.Contains(text, "website:") {
		t.Fatalf("unexpected website:\n%s", text)
	}
}

func TestFoldSnap_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-snapcraft")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\n# snapcraft pack <dir> --output <out>\nout=\"\"\nprev=\"\"\nfor a in \"$@\"; do\n  if [ \"$prev\" = \"--output\" ]; then out=\"$a\"; fi\n  prev=\"$a\"\ndone\nprintf 'SNAP' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_SNAPCRAFT", tool)

	prime := filepath.Join(dir, "prime")
	if err := os.MkdirAll(filepath.Join(prime, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prime, "meta", "snap.yaml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "demo.snap")
	art, err := packaging.FoldSnap(prime, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxSnap || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "SNAP" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildSnap_EndToEndWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-snapcraft")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nout=\"\"\nprev=\"\"\nfor a in \"$@\"; do\n  if [ \"$prev\" = \"--output\" ]; then out=\"$a\"; fi\n  prev=\"$a\"\ndone\nprintf 'BUILTSNAP' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_SNAPCRAFT", tool)

	bin := filepath.Join(dir, "app")
	if err := os.WriteFile(bin, []byte("ELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxSnap},
	}
	out := filepath.Join(dir, "Demo.snap")
	art, err := packaging.BuildSnap(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "BUILTSNAP" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveSnapcraft_Missing(t *testing.T) {
	t.Setenv("VITRA_SNAPCRAFT", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveSnapcraft()
	if err == nil || !strings.Contains(err.Error(), "VITRA_SNAPCRAFT") {
		t.Fatalf("expected missing tool error, got %v", err)
	}
}
