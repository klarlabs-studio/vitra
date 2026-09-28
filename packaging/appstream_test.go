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
		`<url type="donation">https://example.com/demo</url>`,
		`<url type="contact">https://example.com/demo</url>`,
		`<url type="faq">https://example.com/demo</url>`,
		`<url type="contribute">https://example.com/demo</url>`,
		`<url type="translate">https://example.com/demo</url>`,
		`<category>Utility</category>`,
		`<category>Development</category>`,
		`<keyword>desktop</keyword>`,
		`<keyword>secure</keyword>`,
		`<launchable type="desktop-id">com.example.Demo.desktop</launchable>`,
		`<icon type="stock">Demo---Co</icon>`,
		`<pkgname>com-example-demo</pkgname>`,
		`<developer id="com.example">`,
		`<name>Klarlabs &lt;dev@klarlabs.de&gt;</name>`,
		`<developer_name>Klarlabs &lt;dev@klarlabs.de&gt;</developer_name>`,
		`<update_contact>Klarlabs &lt;dev@klarlabs.de&gt;</update_contact>`,
		`<project_group>Vitra</project_group>`,
		`<content_rating type="oars-1.1"/>`,
		`<control>keyboard</control>`,
		`<control>pointing</control>`,
		`<display_length compare="ge">360</display_length>`,
		`<control>touch</control>`,
		`<color type="primary" scheme_preference="light">#eeeeee</color>`,
		`<color type="primary" scheme_preference="dark">#111111</color>`,
		`<release version="1.2.3"/>`,
		`<binary>Demo---Co</binary>`,
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
	if !strings.Contains(emptyXML, `<project_group>Vitra</project_group>`) {
		t.Fatalf("project_group missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<content_rating type="oars-1.1"/>`) {
		t.Fatalf("default OARS content_rating missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<control>keyboard</control>`) || !strings.Contains(emptyXML, `<control>pointing</control>`) {
		t.Fatalf("default recommends controls missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<display_length compare="ge">360</display_length>`) {
		t.Fatalf("default requires display_length missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<control>touch</control>`) {
		t.Fatalf("default supports touch missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<color type="primary" scheme_preference="dark">#111111</color>`) {
		t.Fatalf("default branding primary dark missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<release version="1"/>`) {
		t.Fatalf("release from Version missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<binary>A</binary>`) {
		t.Fatalf("provides binary from Name missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<pkgname>a</pkgname>`) {
		t.Fatalf("pkgname from AppID missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<icon type="stock">A</icon>`) {
		t.Fatalf("stock icon from Name missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<developer id="a">`) {
		t.Fatalf("developer id from single-segment AppID missing:\n%s", emptyXML)
	}
	if !strings.Contains(emptyXML, `<developer_name>A</developer_name>`) {
		t.Fatalf("developer_name from EffectivePublisher missing:\n%s", emptyXML)
	}
	if strings.Contains(emptyXML, `<url type="homepage">`) || strings.Contains(emptyXML, `<url type="help">`) || strings.Contains(emptyXML, `<url type="bugtracker">`) || strings.Contains(emptyXML, `<url type="vcs-browser">`) || strings.Contains(emptyXML, `<url type="donation">`) || strings.Contains(emptyXML, `<url type="contact">`) || strings.Contains(emptyXML, `<url type="faq">`) || strings.Contains(emptyXML, `<url type="contribute">`) || strings.Contains(emptyXML, `<url type="translate">`) {
		t.Fatalf("unexpected homepage/help/bugtracker/vcs-browser/donation/contact/faq/contribute/translate urls when Homepage empty:\n%s", emptyXML)
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
	if !strings.Contains(string(body), `<project_group>Vitra</project_group>`) {
		t.Fatalf("project_group missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<content_rating type="oars-1.1"/>`) {
		t.Fatalf("OARS content_rating missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<control>keyboard</control>`) || !strings.Contains(string(body), `<control>pointing</control>`) {
		t.Fatalf("recommends controls missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<display_length compare="ge">360</display_length>`) {
		t.Fatalf("requires display_length missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<control>touch</control>`) {
		t.Fatalf("supports touch missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<color type="primary" scheme_preference="light">#eeeeee</color>`) {
		t.Fatalf("branding primary light missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<release version="0.1.0"/>`) {
		t.Fatalf("release missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<binary>Vitra-Demo</binary>`) {
		t.Fatalf("provides binary missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<pkgname>com-vitra-demo</pkgname>`) {
		t.Fatalf("pkgname missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<icon type="stock">Vitra-Demo</icon>`) {
		t.Fatalf("stock icon missing:\n%s", body)
	}
	if !strings.Contains(string(body), `<developer id="com.vitra">`) {
		t.Fatalf("developer id missing:\n%s", body)
	}
}
