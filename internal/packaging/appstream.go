package packaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppStreamMetainfoRel returns the FreeDesktop metainfo path under a prefix
// such as "usr/share" or "share" (Flatpak files/).
func AppStreamMetainfoRel(prefix, appID string) string {
	prefix = strings.Trim(prefix, "/")
	id := sanitizeFileName(appID)
	if prefix == "" {
		return "metainfo/" + id + ".metainfo.xml"
	}
	return prefix + "/metainfo/" + id + ".metainfo.xml"
}

// AppStreamMetainfoXML renders a minimal AppStream component for Flathub /
// software centers. It is metadata only — no store submission.
func AppStreamMetainfoXML(spec Spec) string {
	id := sanitizeFileName(spec.AppID)
	name := xmlEscape(spec.Name)
	summary := xmlEscape(spec.EffectiveDescription())
	developer := xmlEscape(spec.EffectivePublisher())
	contact := xmlEscape(spec.EffectiveMaintainer())
	license := xmlEscape(spec.EffectiveLicense())
	desktopID := id + ".desktop"
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>%s</id>
  <metadata_license>CC0-1.0</metadata_license>
  <project_license>%s</project_license>
  <name>%s</name>
  <summary>%s</summary>
  <description>
    <p>%s</p>
  </description>
  <launchable type="desktop-id">%s</launchable>
  <icon type="stock">%s</icon>
  <pkgname>%s</pkgname>
  <developer id="%s">
    <name>%s</name>
  </developer>
  <developer_name>%s</developer_name>
  <update_contact>%s</update_contact>
  <project_group>Vitra</project_group>
`, id, license, name, summary, summary, desktopID, sanitizeFileName(spec.Name), debianName(spec.AppID, sanitizeFileName(spec.Name)), appStreamDeveloperID(spec.AppID), developer, developer, contact)
	if home := strings.TrimSpace(spec.Homepage); home != "" {
		escaped := xmlEscape(home)
		fmt.Fprintf(&b, "  <url type=\"homepage\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"help\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"bugtracker\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"vcs-browser\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"donation\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"contact\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"faq\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"contribute\">%s</url>\n", escaped)
		fmt.Fprintf(&b, "  <url type=\"translate\">%s</url>\n", escaped)
	}
	// Empty OARS 1.1 rating means all attributes are "none" (Flathub-friendly default).
	b.WriteString("  <content_rating type=\"oars-1.1\"/>\n")
	b.WriteString("  <recommends>\n")
	b.WriteString("    <control>keyboard</control>\n")
	b.WriteString("    <control>pointing</control>\n")
	b.WriteString("  </recommends>\n")
	b.WriteString("  <requires>\n")
	b.WriteString("    <display_length compare=\"ge\">360</display_length>\n")
	b.WriteString("  </requires>\n")
	b.WriteString("  <supports>\n")
	b.WriteString("    <control>touch</control>\n")
	b.WriteString("  </supports>\n")
	// Default primary branding matches the Vitra scaffold chrome (dark/light).
	b.WriteString("  <branding>\n")
	b.WriteString("    <color type=\"primary\" scheme_preference=\"light\">#eeeeee</color>\n")
	b.WriteString("    <color type=\"primary\" scheme_preference=\"dark\">#111111</color>\n")
	b.WriteString("  </branding>\n")
	b.WriteString("  <categories>\n")
	for _, cat := range spec.EffectiveCategories() {
		fmt.Fprintf(&b, "    <category>%s</category>\n", xmlEscape(cat))
	}
	b.WriteString("  </categories>\n")
	if kws := spec.EffectiveKeywords(); len(kws) > 0 {
		b.WriteString("  <keywords>\n")
		for _, kw := range kws {
			fmt.Fprintf(&b, "    <keyword>%s</keyword>\n", xmlEscape(kw))
		}
		b.WriteString("  </keywords>\n")
	}
	if ver := strings.TrimSpace(spec.Version); ver != "" {
		b.WriteString("  <releases>\n")
		fmt.Fprintf(&b, "    <release version=\"%s\"/>\n", xmlEscape(ver))
		b.WriteString("  </releases>\n")
	}
	if bin := sanitizeFileName(spec.Name); bin != "" {
		b.WriteString("  <provides>\n")
		fmt.Fprintf(&b, "    <binary>%s</binary>\n", xmlEscape(bin))
		b.WriteString("  </provides>\n")
	}
	b.WriteString("</component>\n")
	return b.String()
}

// appStreamDeveloperID returns a reverse-DNS developer id derived from AppID
// (drops the final component: com.example.Demo → com.example).
func appStreamDeveloperID(appID string) string {
	id := sanitizeFileName(appID)
	if i := strings.LastIndex(id, "."); i > 0 {
		return id[:i]
	}
	return id
}

// writeAppStreamMetainfo writes metainfo XML under root/relPath.
func writeAppStreamMetainfo(spec Spec, root, relPath string) error {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(AppStreamMetainfoXML(spec)), 0o644)
}
