package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestAppStreamMetainfoXML(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.Demo", Version: "1.2.3", Name: "Demo & Co",
		Description: `Hello <world>`,
		Maintainer:  "Klarlabs <dev@klarlabs.de>",
		Homepage:    "https://example.com/demo",
		Categories:  []string{"Utility", "Development"},
		License:     "Apache-2.0",
		Targets:     []packaging.Target{packaging.TargetLinuxDir},
	}
	xml := packaging.AppStreamMetainfoXML(spec)
	for _, want := range []string{
		`<id>com.example.Demo</id>`,
		`<name>Demo &amp; Co</name>`,
		`<summary>Hello &lt;world&gt;</summary>`,
		`<project_license>Apache-2.0</project_license>`,
		`<url type="homepage">https://example.com/demo</url>`,
		`<category>Utility</category>`,
		`<category>Development</category>`,
		`<launchable type="desktop-id">com.example.Demo.desktop</launchable>`,
		`<developer_name>Klarlabs &lt;dev@klarlabs.de&gt;</developer_name>`,
		`type="desktop-application"`,
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("missing %q in %s", want, xml)
		}
	}
	if packaging.AppStreamMetainfoRel("usr/share", "com.example.Demo") != "usr/share/metainfo/com.example.Demo.metainfo.xml" {
		t.Fatal(packaging.AppStreamMetainfoRel("usr/share", "com.example.Demo"))
	}
	if spec.DesktopCategories() != "Utility;Development;" {
		t.Fatalf("desktop cats=%q", spec.DesktopCategories())
	}
	empty := packaging.Spec{AppID: "a", Version: "1", Name: "A", Targets: []packaging.Target{packaging.TargetLinuxDir}}
	if empty.EffectiveLicense() != packaging.DefaultLicense {
		t.Fatal(empty.EffectiveLicense())
	}
	if empty.DesktopCategories() != "Utility;" {
		t.Fatal(empty.DesktopCategories())
	}
}

func TestStageLinux_WritesAppStreamMetainfo(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.WriteFile(bin, []byte("elf"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "stage")
	spec := packaging.Spec{
		AppID: "com.vitra.demo", Version: "0.1.0", Name: "Vitra Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDir},
	}
	if _, err := packaging.StageLinux(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(out, "usr", "share", "metainfo", "com.vitra.demo.metainfo.xml")
	body, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<id>com.vitra.demo</id>") {
		t.Fatalf("%s", body)
	}
}
