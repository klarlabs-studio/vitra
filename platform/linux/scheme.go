package linux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var schemeName = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

func dataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func desktopFileName(appID string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, appID)
	return safe + "-url.desktop"
}

// RegisterURLScheme installs a user-local FreeDesktop handler for scheme
// (e.g. "vitra") so OS open of scheme://… launches execPath with the URL.
// Writes ~/.local/share/applications/<appID>-url.desktop and, when available,
// runs xdg-mime default. Does not require the WebView host.
func (h *Host) RegisterURLScheme(scheme, appID, execPath string) error {
	if !schemeName.MatchString(scheme) {
		return fmt.Errorf("invalid URL scheme %q", scheme)
	}
	if appID == "" {
		return fmt.Errorf("app id required for URL scheme registration")
	}
	if execPath == "" {
		return fmt.Errorf("executable path required for URL scheme registration")
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
	desktop := filepath.Join(apps, desktopFileName(appID))
	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s %%u
MimeType=x-scheme-handler/%s;
NoDisplay=true
StartupNotify=false
`, appID, abs, scheme)
	if err := os.WriteFile(desktop, []byte(body), 0o644); err != nil {
		return err
	}
	if path, err := exec.LookPath("xdg-mime"); err == nil {
		cmd := exec.Command(path, "default", filepath.Base(desktop), "x-scheme-handler/"+scheme)
		// Best-effort: headless / CI environments often lack a mime database session.
		_ = cmd.Run()
	}
	if path, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(path, apps).Run()
	}
	return nil
}

// UnregisterURLScheme removes the user-local desktop handler for appID.
func (h *Host) UnregisterURLScheme(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	home, err := dataHome()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "applications", desktopFileName(appID))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
