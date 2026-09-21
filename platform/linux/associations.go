package linux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var mimeTypeName = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9.+]*/[a-zA-Z0-9][-a-zA-Z0-9.+]*$`)

func filesDesktopName(appID string) string {
	return strings.TrimSuffix(desktopFileName(appID), "-url.desktop") + "-files.desktop"
}

// RegisterFileAssociations installs a user-local FreeDesktop handler so the
// given MIME types open with execPath. Writes
// ~/.local/share/applications/<appID>-files.desktop and best-effort xdg-mime.
// Does not require the WebView host.
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
	home, err := dataHome()
	if err != nil {
		return err
	}
	apps := filepath.Join(home, "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return err
	}
	desktopName := filesDesktopName(appID)
	desktop := filepath.Join(apps, desktopName)
	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s %%f
MimeType=%s;
NoDisplay=true
StartupNotify=false
`, name, abs, strings.Join(types, ";"))
	if err := os.WriteFile(desktop, []byte(body), 0o644); err != nil {
		return err
	}
	if path, err := exec.LookPath("xdg-mime"); err == nil {
		for _, m := range types {
			_ = exec.Command(path, "default", desktopName, m).Run()
		}
	}
	if path, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(path, apps).Run()
	}
	return nil
}

// UnregisterFileAssociations removes the user-local file-association desktop file.
func (h *Host) UnregisterFileAssociations(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	home, err := dataHome()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "applications", filesDesktopName(appID))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
