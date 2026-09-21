package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestFoldDMG_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-hdiutil")
	script := `#!/bin/sh
out=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-format" ]; then :; fi
  prev="$a"
done
# last arg is output path for hdiutil create
out="$last"
last=""
for a in "$@"; do last="$a"; done
printf 'DMG' > "$last"
`
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_HDIUTIL", tool)

	app := filepath.Join(dir, "Demo.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Demo.dmg")
	art, err := packaging.FoldDMG(app, "Demo", out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetDarwinDMG || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "DMG" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildDMG_EndToEndWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-hdiutil")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nlast=\"\"\nfor a in \"$@\"; do last=\"$a\"; done\nprintf 'BUILTDMG' > \"$last\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_HDIUTIL", tool)

	bin := filepath.Join(dir, "app.bin")
	if err := os.WriteFile(bin, []byte("mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetDarwinDMG},
	}
	out := filepath.Join(dir, "Demo.dmg")
	art, err := packaging.BuildDMG(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "BUILTDMG" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveHdiutil_Missing(t *testing.T) {
	t.Setenv("VITRA_HDIUTIL", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveHdiutil()
	if err == nil || !strings.Contains(err.Error(), "VITRA_HDIUTIL") {
		t.Fatalf("expected missing hdiutil error, got %v", err)
	}
}

func TestFoldDMG_RequiresAppBundle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VITRA_HDIUTIL", filepath.Join(dir, "unused"))
	_, err := packaging.FoldDMG(dir, "X", filepath.Join(dir, "x.dmg"))
	if err == nil || !strings.Contains(err.Error(), ".app") {
		t.Fatalf("got %v", err)
	}
}
