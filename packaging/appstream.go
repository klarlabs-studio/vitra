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
	desktopID := id + ".desktop"
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>%s</id>
  <metadata_license>CC0-1.0</metadata_license>
  <project_license>LicenseRef-proprietary</project_license>
  <name>%s</name>
  <summary>%s</summary>
  <description>
    <p>%s</p>
  </description>
  <launchable type="desktop-id">%s</launchable>
  <developer_name>%s</developer_name>
  <categories>
    <category>Utility</category>
  </categories>
</component>
`, id, name, summary, summary, desktopID, developer)
}

// writeAppStreamMetainfo writes metainfo XML under root/relPath.
func writeAppStreamMetainfo(spec Spec, root, relPath string) error {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(AppStreamMetainfoXML(spec)), 0o644)
}
