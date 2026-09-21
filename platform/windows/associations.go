package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mimeTypeName = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9.+]*/[a-zA-Z0-9][-a-zA-Z0-9.+]*$`)

// RegisterFileAssociations writes an HKCU ProgID + MIME .reg helper and
// best-effort imports it. Does not require the native WebView2 host.
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
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	progID := safeName(appID) + ".file"
	regPath := filepath.Join(root, safeName(appID)+"-files.reg")
	cmd := fmt.Sprintf("\"%s\" \"%%1\"", strings.ReplaceAll(abs, `"`, `\"`))
	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\n\n")
	fmt.Fprintf(&b, "[HKEY_CURRENT_USER\\Software\\Classes\\%s]\n", progID)
	fmt.Fprintf(&b, "@=\"%s\"\n\n", regEscape(name))
	fmt.Fprintf(&b, "[HKEY_CURRENT_USER\\Software\\Classes\\%s\\shell\\open\\command]\n", progID)
	fmt.Fprintf(&b, "@=\"%s\"\n\n", regEscape(cmd))
	for _, m := range types {
		fmt.Fprintf(&b, "[HKEY_CURRENT_USER\\Software\\Classes\\MIME\\Database\\Content Type\\%s]\n", m)
		fmt.Fprintf(&b, "\"ProgID\"=\"%s\"\n\n", progID)
	}
	if err := os.WriteFile(regPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	registryImportRunner(regPath)
	return nil
}

// UnregisterFileAssociations removes the helper .reg file for appID.
func (h *Host) UnregisterFileAssociations(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	root, err := supportHome()
	if err != nil {
		return err
	}
	path := filepath.Join(root, safeName(appID)+"-files.reg")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
