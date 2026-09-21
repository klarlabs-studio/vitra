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
		Keywords:    []string{"desktop", "secure"},
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
		`<url type="help">https://example.com/demo</url>`,
		`<url type="bugtracker">https://example.com/demo</url>`,
		`<url type="vcs-browser">https://example.com/demo</url>`,
		`<category>Utility</category>`,
		`<category>Development</category>`,
		`<keyword>desktop</keyword>`,
		`<keyword>secure</keyword>`,
		`<launchable type="desktop-id">com.example.Demo.desktop</launchable>`,
		`<developer_name>Klarlabs &lt;dev@klarlabs.de&gt;</developer_name>`,
		`<update_contact>Klarlabs &lt;dev@klarlabs.de&gt;</update_contact>`,
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
	if spec.DesktopKeywords() != "desktop;secure;" || spec.DesktopKeywordsLine() != "Keywords=desktop;secure;\n" {
		t.Fatalf("desktop keywords=%q line=%q", spec.DesktopKeywords(), spec.DesktopKeywordsLine())
	}
	empty := packaging.Spec{AppID: "a", Version: "1", Name: "A", Targets: []packaging.Target{packaging.TargetLinuxDir}}
	if empty.EffectiveLicense() != packaging.DefaultLicense {
		t.Fatal(empty.EffectiveLicense())
	}
	if empty.DesktopCategories() != "Utility;" {
		t.Fatal(empty.DesktopCategories())
	}
	if empty.DesktopKeywordsLine() != "" {
		t.Fatalf("expected empty keywords line, got %q", empty.DesktopKeywordsLine())
	}
	emptyXML := packaging.AppStreamMetainfoXML(empty)
	if !strings.Contains(emptyXML, `<update_contact>Vitra Packaging &lt;vitra@klarlabs.de&gt;</update_contact>`) {
		t.Fatalf("default update_contact missing:\n%s", emptyXML)
	}
	if strings.Contains(emptyXML, `<url type="homepage">`) || strings.Contains(emptyXML, `<url type="help">`) || strings.Contains(emptyXML, `<url type="bugtracker">`) || strings.Contains(emptyXML, `<url type="vcs-browser">`) {
		t.Fatalf("unexpected homepage/help/bugtracker/vcs-browser urls when Homepage empty:\n%s", emptyXML)
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
	if !strings.Contains(string(body), `<update_contact>Vitra Packaging &lt;vitra@klarlabs.de&gt;</update_contact>`) {
		t.Fatalf("default update_contact missing:\n%s", body)
	}
}
