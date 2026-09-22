package packaging_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
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
	desktop, _ := os.ReadFile(filepath.Join(out, "Vitra-Demo.desktop"))
	if !strings.Contains(string(desktop), "StartupWMClass=Vitra-Demo") {
		t.Fatalf("desktop=%s", desktop)
	}
	if !strings.Contains(string(desktop), "X-GNOME-UsesNotifications=true") {
		t.Fatalf("desktop missing X-GNOME-UsesNotifications=%s", desktop)
	}
	if !strings.Contains(string(desktop), "StartupNotify=true") {
		t.Fatalf("desktop missing StartupNotify=%s", desktop)
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

func TestBuildDeb_StagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	icon := filepath.Join(tmp, "app.png")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("PNGICON"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64", IconPath: icon,
	}
	if _, err := packaging.BuildDeb(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	files := debDataFiles(t, out)
	pixmap := files["usr/share/pixmaps/Vitra-Demo.png"]
	if string(pixmap) != "PNGICON" {
		t.Fatalf("pixmap=%q", pixmap)
	}
	hicolor := files["usr/share/icons/hicolor/256x256/apps/Vitra-Demo.png"]
	if string(hicolor) != "PNGICON" {
		t.Fatalf("hicolor=%q", hicolor)
	}
	desktop := string(files["usr/share/applications/com.vitra.demo.desktop"])
	if !strings.Contains(desktop, "Icon=Vitra-Demo") {
		t.Fatalf("desktop:\n%s", desktop)
	}
	if !strings.Contains(desktop, "StartupWMClass=Vitra-Demo") {
		t.Fatalf("desktop missing StartupWMClass:\n%s", desktop)
	}
	if !strings.Contains(desktop, "X-GNOME-UsesNotifications=true") {
		t.Fatalf("desktop missing X-GNOME-UsesNotifications:\n%s", desktop)
	}
	if !strings.Contains(desktop, "StartupNotify=true") {
		t.Fatalf("desktop missing StartupNotify:\n%s", desktop)
	}
}

func TestBuildDeb_SectionFromCategories(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64", Categories: []string{"Development", "Utility"},
	}
	if _, err := packaging.BuildDeb(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	control := debControlFile(t, out)
	if !strings.Contains(control, "Section: devel\n") {
		t.Fatalf("expected Section: devel\n%s", control)
	}

	outDefault := filepath.Join(tmp, "vitra-default.deb")
	spec.Categories = nil
	if _, err := packaging.BuildDeb(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	control = debControlFile(t, outDefault)
	if !strings.Contains(control, "Section: utils\n") {
		t.Fatalf("expected default Section: utils\n%s", control)
	}
}

func TestSpec_DebianSection(t *testing.T) {
	cases := []struct {
		cats []string
		want string
	}{
		{nil, "utils"},
		{[]string{"Utility"}, "utils"},
		{[]string{"Development"}, "devel"},
		{[]string{"Network", "Utility"}, "net"},
		{[]string{"UnknownCat"}, "utils"},
	}
	for _, tc := range cases {
		got := packaging.Spec{Categories: tc.cats}.DebianSection()
		if got != tc.want {
			t.Fatalf("cats=%v: got %q want %q", tc.cats, got, tc.want)
		}
	}
}

func TestBuildDeb_Copyright(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64", License: "Apache-2.0", Homepage: "https://example.com/demo",
		Maintainer: "Acme Labs <packaging@example.com>",
	}
	if _, err := packaging.BuildDeb(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	files := debDataFiles(t, out)
	body := string(files["usr/share/doc/com-vitra-demo/copyright"])
	for _, want := range []string{
		"Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/",
		"Upstream-Name: Vitra Demo",
		"Upstream-Contact: Acme Labs <packaging@example.com>",
		"Source: https://example.com/demo",
		"Files: *",
		"License: Apache-2.0",
		"/usr/share/common-licenses/Apache-2.0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}

	outDefault := filepath.Join(tmp, "vitra-default.deb")
	spec.License = ""
	spec.Homepage = ""
	if _, err := packaging.BuildDeb(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	files = debDataFiles(t, outDefault)
	body = string(files["usr/share/doc/com-vitra-demo/copyright"])
	if !strings.Contains(body, "License: "+packaging.DefaultLicense) {
		t.Fatalf("default license missing\n%s", body)
	}
	if strings.Contains(body, "Source:") {
		t.Fatalf("unexpected Source when Homepage empty\n%s", body)
	}
}

func TestDebianCopyright_SPDXDefault(t *testing.T) {
	body := packaging.DebianCopyright(packaging.Spec{
		Name: "Demo", License: "MIT", Maintainer: "Demo <demo@example.com>",
	})
	if !strings.Contains(body, "https://spdx.org/licenses/MIT.html") {
		t.Fatalf("spdx link missing\n%s", body)
	}
}

func TestBuildDeb_CustomDescription(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64", Description: "Custom demo package for CI.",
	}
	if _, err := packaging.BuildDeb(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	control := debControlFile(t, out)
	if !strings.Contains(control, " Custom demo package for CI.") {
		t.Fatalf("control:\n%s", control)
	}
	files := debDataFiles(t, out)
	desktop := string(files["usr/share/applications/com.vitra.demo.desktop"])
	if !strings.Contains(desktop, "Comment=Custom demo package for CI.") {
		t.Fatalf("desktop:\n%s", desktop)
	}
}

func TestSpec_EffectiveDescription(t *testing.T) {
	s := packaging.Spec{Name: "Demo"}
	if s.EffectiveDescription() != packaging.DefaultDescription {
		t.Fatalf("default: %q", s.EffectiveDescription())
	}
	s.Description = "Hello"
	if s.EffectiveDescription() != "Hello" {
		t.Fatalf("custom: %q", s.EffectiveDescription())
	}
}

func TestBuildDeb_CustomMaintainer(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "vitra.deb")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.3.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
		Arch:    "amd64", Maintainer: "Acme <packaging@acme.example>",
	}
	if _, err := packaging.BuildDeb(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	control := debControlFile(t, out)
	if !strings.Contains(control, "Maintainer: Acme <packaging@acme.example>") {
		t.Fatalf("control:\n%s", control)
	}
}

func TestSpec_EffectiveMaintainerPublisher(t *testing.T) {
	s := packaging.Spec{Name: "Demo"}
	if s.EffectiveMaintainer() != packaging.DefaultMaintainer {
		t.Fatalf("default maintainer: %q", s.EffectiveMaintainer())
	}
	if s.EffectivePublisher() != "Demo" {
		t.Fatalf("default publisher: %q", s.EffectivePublisher())
	}
	s.Maintainer = "Acme <a@b.c>"
	if s.EffectiveMaintainer() != "Acme <a@b.c>" || s.EffectivePublisher() != "Acme <a@b.c>" {
		t.Fatalf("custom: maint=%q pub=%q", s.EffectiveMaintainer(), s.EffectivePublisher())
	}
}

func TestBuildDeb_IconMissing(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "payload")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets:  []packaging.Target{packaging.TargetLinuxDeb},
		IconPath: filepath.Join(tmp, "missing.png"),
	}
	_, err := packaging.BuildDeb(spec, bin, filepath.Join(tmp, "x.deb"))
	if err == nil || !strings.Contains(err.Error(), "icon") {
		t.Fatalf("got %v", err)
	}
}

func debControlFile(t *testing.T, debPath string) string {
	t.Helper()
	files := debArMemberFiles(t, debPath, "control.tar.gz")
	body, ok := files["control"]
	if !ok {
		t.Fatal("missing control in control.tar.gz")
	}
	return string(body)
}

func debDataFiles(t *testing.T, debPath string) map[string][]byte {
	t.Helper()
	return debArMemberFiles(t, debPath, "data.tar.gz")
}

func debArMemberFiles(t *testing.T, debPath, member string) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(debPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("!<arch>\n")) {
		t.Fatal("not an ar archive")
	}
	pos := 8
	var tgz []byte
	for pos+60 <= len(raw) {
		hdr := raw[pos : pos+60]
		name := strings.TrimSpace(string(hdr[0:16]))
		size := 0
		for _, c := range strings.TrimSpace(string(hdr[48:58])) {
			size = size*10 + int(c-'0')
		}
		pos += 60
		body := raw[pos : pos+size]
		pos += size
		if size%2 == 1 {
			pos++
		}
		if name == member {
			tgz = body
			break
		}
	}
	if tgz == nil {
		t.Fatalf("missing %s", member)
	}
	gr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gr.Close() }()
	tr := tar.NewReader(gr)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out[hdr.Name] = b
	}
	return out
}
