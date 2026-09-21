package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestBuildFlatpakDir_Layout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(tmp, "logo.png")
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets:     []packaging.Target{packaging.TargetLinuxFlatpak},
		IconPath:    icon,
		Description: "Demo Flatpak",
	}
	art, err := packaging.BuildFlatpakDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxFlatpak || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	for _, p := range []string{
		filepath.Join(out, "files", "bin", "Vitra-Demo"),
		filepath.Join(out, "files", "share", "applications", "com.vitra.demo.desktop"),
		filepath.Join(out, "files", "share", "metainfo", "com.vitra.demo.metainfo.xml"),
		filepath.Join(out, "files", "share", "icons", "hicolor", "256x256", "apps", "Vitra-Demo.png"),
		filepath.Join(out, "metadata"),
		filepath.Join(out, "manifest.yml"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	meta, _ := os.ReadFile(filepath.Join(out, "metadata"))
	for _, want := range []string{
		"name=com.vitra.demo", "command=Vitra-Demo", "org.freedesktop.Platform/",
		"[Context]", "shared=network;ipc;", "[Session Bus Policy]",
		"org.freedesktop.portal.Desktop=talk", "org.freedesktop.portal.FileChooser=talk",
	} {
		if !strings.Contains(string(meta), want) {
			t.Fatalf("metadata missing %q:\n%s", want, meta)
		}
	}
	manifest, _ := os.ReadFile(filepath.Join(out, "manifest.yml"))
	for _, want := range []string{
		"app-id: com.vitra.demo", "runtime-version: \"23.08\"", "command: Vitra-Demo",
		"cp -a files/. /app/", "--talk-name=org.freedesktop.portal.Desktop",
		"--talk-name=org.freedesktop.portal.FileChooser", "--share=network",
	} {
		if !strings.Contains(string(manifest), want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
	if strings.Contains(string(manifest), "--filesystem=home") {
		t.Fatalf("manifest should prefer portals over --filesystem=home:\n%s", manifest)
	}
}

func TestFoldFlatpak_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-flatpak")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'FLATPAK' > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_FLATPAK_BUILDER", tool)

	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "metadata"), []byte("[Application]\nname=com.ex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "demo.flatpak")
	art, err := packaging.FoldFlatpak(stage, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxFlatpak || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "FLATPAK" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildFlatpak_EndToEndWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-flatpak")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'BUILTFP' > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_FLATPAK_BUILDER", tool)

	bin := filepath.Join(dir, "app")
	if err := os.WriteFile(bin, []byte("ELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxFlatpak},
	}
	out := filepath.Join(dir, "Demo.flatpak")
	art, err := packaging.BuildFlatpak(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "BUILTFP" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveFlatpakBuilder_Missing(t *testing.T) {
	t.Setenv("VITRA_FLATPAK_BUILDER", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveFlatpakBuilder()
	if err == nil || !strings.Contains(err.Error(), "VITRA_FLATPAK_BUILDER") {
		t.Fatalf("expected missing tool error, got %v", err)
	}
}
