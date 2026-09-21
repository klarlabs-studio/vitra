package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestFoldMSI_UsesInjectedTools(t *testing.T) {
	dir := t.TempDir()
	candle := filepath.Join(dir, "fake-candle")
	light := filepath.Join(dir, "fake-light")
	if err := os.WriteFile(candle, []byte("#!/bin/sh\n# candle stub\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(light, []byte("#!/bin/sh\n# $args include -out <msi>\nout=\"\"\nprev=\"\"\nfor a in \"$@\"; do\n  if [ \"$prev\" = \"-out\" ]; then out=\"$a\"; fi\n  prev=\"$a\"\ndone\nprintf 'MSI' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_CANDLE", candle)
	t.Setenv("VITRA_LIGHT", light)

	wixDir := filepath.Join(dir, "stage")
	if err := os.MkdirAll(filepath.Join(wixDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wixDir, "product.wxs"), []byte("<Wix/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Demo.msi")
	art, err := packaging.FoldMSI(wixDir, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetWindowsMSI || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "MSI" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildMSI_EndToEndWithFakeTools(t *testing.T) {
	dir := t.TempDir()
	candle := filepath.Join(dir, "fake-candle")
	light := filepath.Join(dir, "fake-light")
	if err := os.WriteFile(candle, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(light, []byte("#!/bin/sh\nout=\"\"\nprev=\"\"\nfor a in \"$@\"; do\n  if [ \"$prev\" = \"-out\" ]; then out=\"$a\"; fi\n  prev=\"$a\"\ndone\nprintf 'BUILTMSI' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_CANDLE", candle)
	t.Setenv("VITRA_LIGHT", light)

	bin := filepath.Join(dir, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetWindowsMSI},
	}
	out := filepath.Join(dir, "Demo.msi")
	art, err := packaging.BuildMSI(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "BUILTMSI" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveCandle_Missing(t *testing.T) {
	t.Setenv("VITRA_CANDLE", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveCandle()
	if err == nil || !strings.Contains(err.Error(), "VITRA_CANDLE") {
		t.Fatalf("expected missing candle error, got %v", err)
	}
}

func TestFoldNSIS_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-makensis")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\n# last -XOutFile arg or last arg\nout=\"\"\nfor a in \"$@\"; do\n  case \"$a\" in\n    -XOutFile*) out=\"${a#-XOutFile }\" ;;\n  esac\ndone\nif [ -z \"$out\" ]; then out=\"$2\"; fi\nprintf 'NSIS' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_MAKENSIS", tool)

	nsisDir := filepath.Join(dir, "stage")
	if err := os.MkdirAll(filepath.Join(nsisDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nsisDir, "installer.nsi"), []byte("OutFile x.exe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Demo-setup.exe")
	art, err := packaging.FoldNSIS(nsisDir, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetWindowsNSIS || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "NSIS" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestResolveMakensis_Missing(t *testing.T) {
	t.Setenv("VITRA_MAKENSIS", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveMakensis()
	if err == nil || !strings.Contains(err.Error(), "VITRA_MAKENSIS") {
		t.Fatalf("expected missing makensis error, got %v", err)
	}
}
