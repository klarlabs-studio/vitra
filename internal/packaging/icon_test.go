package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHicolorIconRel(t *testing.T) {
	if got := hicolorIconRel("App.png"); got != "usr/share/icons/hicolor/256x256/apps/App.png" {
		t.Fatalf("png: %q", got)
	}
	if got := hicolorIconRel("App.svg"); got != "usr/share/icons/hicolor/scalable/apps/App.svg" {
		t.Fatalf("svg: %q", got)
	}
}

func TestStageIconFile_CopiesPNG(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "logo.png")
	if err := os.WriteFile(src, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "out")
	key, name, err := stageIconFile(src, dest, "Demo")
	if err != nil {
		t.Fatal(err)
	}
	if key != "Demo" || name != "Demo.png" {
		t.Fatalf("key=%q name=%q", key, name)
	}
	got, err := os.ReadFile(filepath.Join(dest, name))
	if err != nil || string(got) != "PNGDATA" {
		t.Fatalf("%q err=%v", got, err)
	}
}

func TestStageIconFile_EmptyNoop(t *testing.T) {
	key, name, err := stageIconFile("", t.TempDir(), "X")
	if err != nil || key != "" || name != "" {
		t.Fatalf("key=%q name=%q err=%v", key, name, err)
	}
}

func TestStageIconFile_RejectsUnknownExt(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "logo.bmp")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := stageIconFile(src, tmp, "X")
	if err == nil || !strings.Contains(err.Error(), "unsupported icon") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildAppDir_StagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.png")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "Demo.AppDir")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []Target{TargetLinuxAppImage}, IconPath: icon,
	}
	if _, err := BuildAppDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "Demo.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(out, ".DirIcon")); err != nil {
		t.Fatal(err)
	}
	hicolor := filepath.Join(out, "usr", "share", "icons", "hicolor", "256x256", "apps", "Demo.png")
	if _, err := os.Stat(hicolor); err != nil {
		t.Fatal(err)
	}
}

func TestStageLinux_StagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.png")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []Target{TargetLinuxDir}, IconPath: icon,
	}
	if _, err := StageLinux(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "Demo.png")); err != nil {
		t.Fatal(err)
	}
	hicolor := filepath.Join(out, "usr", "share", "icons", "hicolor", "256x256", "apps", "Demo.png")
	if _, err := os.Stat(hicolor); err != nil {
		t.Fatal(err)
	}
}

func TestStageDarwinApp_StagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.icns")
	if err := os.WriteFile(bin, []byte("mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("icns"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []Target{TargetDarwinApp}, IconPath: icon,
	}
	art, err := StageDarwinApp(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(art.Path, "Contents", "Resources", "Demo.icns")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(art.Path, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "CFBundleIconFile") || !strings.Contains(body, "<string>Demo</string>") {
		t.Fatalf("plist:\n%s", body)
	}
}
