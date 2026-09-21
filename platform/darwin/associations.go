package darwin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mimeTypeName = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9.+]*/[a-zA-Z0-9][-a-zA-Z0-9.+]*$`)

// RegisterFileAssociations installs a user-local helper .app with
// CFBundleDocumentTypes for the given MIME types. Best-effort lsregister.
// Does not require WKWebView.
func (h *Host) RegisterFileAssociations(appID, execPath, name string, mimeTypes []string) error {
	if appID == "" {
		return fmt.Errorf("app id required for file association")
	}
	if execPath == "" {
		return fmt.Errorf("executable path required for file association")
	}
	if name == "" {
		name = appID
	}
	if len(mimeTypes) == 0 {
		return fmt.Errorf("at least one MIME type is required")
	}
	seen := map[string]struct{}{}
	var types []string
	for _, m := range mimeTypes {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if !mimeTypeName.MatchString(m) {
			return fmt.Errorf("invalid MIME type %q", m)
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		types = append(types, m)
	}
	if len(types) == 0 {
		return fmt.Errorf("at least one MIME type is required")
	}
	abs, err := filepath.Abs(execPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	root, err := supportHome()
	if err != nil {
		return err
	}
	bundle := filepath.Join(root, safeBundleName(appID)+"-files.app")
	_ = os.RemoveAll(bundle)
	var mimeXML strings.Builder
	for _, m := range types {
		mimeXML.WriteString("\t\t\t\t<string>")
		mimeXML.WriteString(xmlEscape(m))
		mimeXML.WriteString("</string>\n")
	}
	extra := fmt.Sprintf(`	<key>CFBundleDocumentTypes</key>
	<array>
		<dict>
			<key>CFBundleTypeName</key>
			<string>%s</string>
			<key>CFBundleTypeRole</key>
			<string>Viewer</string>
			<key>CFBundleTypeMIMETypes</key>
			<array>
%s			</array>
		</dict>
	</array>
`, xmlEscape(name), mimeXML.String())
	if err := writeHelperApp(bundle, appID+".files", name, abs, extra); err != nil {
		return err
	}
	lsregisterRunner(bundle)
	return nil
}

// UnregisterFileAssociations removes the helper .app for appID file types.
func (h *Host) UnregisterFileAssociations(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	root, err := supportHome()
	if err != nil {
		return err
	}
	path := filepath.Join(root, safeBundleName(appID)+"-files.app")
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
