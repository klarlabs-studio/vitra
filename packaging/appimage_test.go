package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestFoldAppDir_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-appimagetool")
	script := "#!/bin/sh\n# $1=AppDir $2=out\nprintf 'AI' > \"$2\"\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_APPIMAGETOOL", tool)

	appDir := filepath.Join(dir, "Demo.AppDir")
	if err := os.MkdirAll(filepath.Join(appDir, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Demo.AppImage")
	art, err := packaging.FoldAppDir(appDir, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxAppImage || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "AI" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildAppImage_EndToEndWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-appimagetool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\ncp -r \"$1\" \"$2.dir\"\nprintf 'APPIMAGE' > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_APPIMAGETOOL", tool)

	bin := filepath.Join(dir, "app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxAppImage},
	}
	out := filepath.Join(dir, "Demo.AppImage")
	art, err := packaging.BuildAppImage(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "APPIMAGE" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveAppImageTool_Missing(t *testing.T) {
	t.Setenv("VITRA_APPIMAGETOOL", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveAppImageTool()
	if err == nil || !strings.Contains(err.Error(), "VITRA_APPIMAGETOOL") {
		t.Fatalf("expected missing tool error, got %v", err)
	}
}
