package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestBuildRPMDir_Layout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(tmp, "logo.png")
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "rpm-top")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets:  []packaging.Target{packaging.TargetLinuxRPM},
		IconPath: icon, Description: "Demo RPM", Maintainer: "Packager <p@example.com>",
		Homepage: "https://example.com/demo", License: "Apache-2.0",
	}
	art, err := packaging.BuildRPMDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxRPM || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	for _, p := range []string{
		filepath.Join(out, "payload", "usr", "bin", "Vitra-Demo"),
		filepath.Join(out, "payload", "usr", "share", "applications", "com.vitra.demo.desktop"),
		filepath.Join(out, "payload", "usr", "share", "pixmaps", "Vitra-Demo.png"),
		filepath.Join(out, "SPECS", "com-vitra-demo.spec"),
		filepath.Join(out, "BUILD"),
		filepath.Join(out, "RPMS"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	specBody, err := os.ReadFile(filepath.Join(out, "SPECS", "com-vitra-demo.spec"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(specBody)
	for _, want := range []string{
		"Name: com-vitra-demo",
		"Version: 0.3.0",
		"Release: 1",
		"License: Apache-2.0",
		"Group: Applications/System",
		"URL: https://example.com/demo",
		"Packager: Packager <p@example.com>",
		"BuildArch: ",
		"%description",
		"Demo RPM",
		"%install",
		"%{_topdir}/payload/",
		"/usr/bin/Vitra-Demo",
		"/usr/share/applications/com.vitra.demo.desktop",
		"/usr/share/pixmaps/Vitra-Demo.png",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("spec missing %q:\n%s", want, text)
		}
	}
	desktop, _ := os.ReadFile(filepath.Join(out, "payload", "usr", "share", "applications", "com.vitra.demo.desktop"))
	if !strings.Contains(string(desktop), "StartupWMClass=Vitra-Demo") {
		t.Fatalf("desktop=%s", desktop)
	}
}

func TestBuildRPMDir_DefaultLicenseOmitsURL(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "rpm-top")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxRPM},
	}
	if _, err := packaging.BuildRPMDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out, "SPECS", "com-vitra-demo.spec"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "License: "+packaging.DefaultLicense) {
		t.Fatalf("default license missing:\n%s", text)
	}
	if strings.Contains(text, "URL:") {
		t.Fatalf("unexpected URL:\n%s", text)
	}
	if !strings.Contains(text, "Group: Applications/System") {
		t.Fatalf("default Group missing:\n%s", text)
	}
}

func TestBuildRPMDir_GroupFromCategories(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "rpm-top")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets:    []packaging.Target{packaging.TargetLinuxRPM},
		Categories: []string{"Development", "Utility"},
	}
	if _, err := packaging.BuildRPMDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "SPECS", "com-vitra-demo.spec"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Group: Development/Tools") {
		t.Fatalf("Group missing:\n%s", raw)
	}
}

func TestSpec_RPMGroup(t *testing.T) {
	cases := []struct {
		cats []string
		want string
	}{
		{nil, "Applications/System"},
		{[]string{"Utility"}, "Applications/System"},
		{[]string{"Development"}, "Development/Tools"},
		{[]string{"Network"}, "Applications/Internet"},
		{[]string{"UnknownCat"}, "Applications/System"},
	}
	for _, tc := range cases {
		got := packaging.Spec{Categories: tc.cats}.RPMGroup()
		if got != tc.want {
			t.Fatalf("cats=%v: got %q want %q", tc.cats, got, tc.want)
		}
	}
}

func TestFoldRPM_UsesInjectedTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-rpmbuild")
	script := `#!/bin/sh
top=""
prev=""
for a in "$@"; do
  if [ "$prev" = "--define" ]; then
    case "$a" in
      _topdir\ *) top="${a#_topdir }" ;;
    esac
  fi
  prev="$a"
done
mkdir -p "$top/RPMS/x86_64"
printf 'RPM' > "$top/RPMS/x86_64/demo.rpm"
`
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_RPMBUILD", tool)

	top := filepath.Join(dir, "top")
	if err := os.MkdirAll(filepath.Join(top, "SPECS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(top, "SPECS", "demo.spec"), []byte("Name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "demo.rpm")
	art, err := packaging.FoldRPM(top, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != packaging.TargetLinuxRPM || art.Path != out || art.SHA256 == "" {
		t.Fatalf("%+v", art)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "RPM" {
		t.Fatalf("out=%q err=%v", got, err)
	}
}

func TestBuildRPM_EndToEndWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake-rpmbuild")
	if err := os.WriteFile(tool, []byte(`#!/bin/sh
top=""
prev=""
for a in "$@"; do
  if [ "$prev" = "--define" ]; then
    case "$a" in
      _topdir\ *) top="${a#_topdir }" ;;
    esac
  fi
  prev="$a"
done
mkdir -p "$top/RPMS/noarch"
printf 'BUILTRPM' > "$top/RPMS/noarch/out.rpm"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_RPMBUILD", tool)

	bin := filepath.Join(dir, "app")
	if err := os.WriteFile(bin, []byte("ELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0-rc1", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxRPM},
	}
	out := filepath.Join(dir, "Demo.rpm")
	art, err := packaging.BuildRPM(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil || string(raw) != "BUILTRPM" {
		t.Fatalf("art=%+v raw=%q err=%v", art, raw, err)
	}
}

func TestResolveRpmbuild_Missing(t *testing.T) {
	t.Setenv("VITRA_RPMBUILD", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveRpmbuild()
	if err == nil || !strings.Contains(err.Error(), "VITRA_RPMBUILD") {
		t.Fatalf("expected missing tool error, got %v", err)
	}
}

func TestRPMVersionRelease(t *testing.T) {
	// Exercised via BuildRPMDir Version field; ensure hyphen splits.
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "b")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "top")
	spec := packaging.Spec{
		AppID: "com.ex", Version: "2.0.0-beta", Name: "Ex",
		Targets: []packaging.Target{packaging.TargetLinuxRPM},
	}
	if _, err := packaging.BuildRPMDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(out, "SPECS", "com-ex.spec"))
	if !strings.Contains(string(body), "Version: 2.0.0\n") || !strings.Contains(string(body), "Release: beta\n") {
		t.Fatalf("spec=%s", body)
	}
}
