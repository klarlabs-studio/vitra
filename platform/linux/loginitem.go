package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/vitra/domain"
)

// configHome is $XDG_CONFIG_HOME, or ~/.config.
func configHome() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// autostartFile is the XDG autostart entry of appID.
func autostartFile(appID string) (string, error) {
	if appID == "" {
		return "", &domain.ErrValidation{Message: "app id required for a login item"}
	}
	dir, err := configHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", strings.TrimSuffix(desktopFileName(appID), "-url.desktop")+".desktop"), nil
}

// LoginItemEnabled reports whether the app has an XDG autostart entry.
func (h *Host) LoginItemEnabled(appID string) (bool, error) {
	path, err := autostartFile(appID)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SetLoginItem writes (or removes) an XDG autostart entry that runs
// execPath when the user logs in. Desktops that follow the XDG autostart
// specification (GNOME, KDE Plasma, XFCE, …) honour it. It does not need the
// WebView host.
func (h *Host) SetLoginItem(appID, execPath string, enabled bool) error {
	path, err := autostartFile(appID)
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if execPath == "" {
		return &domain.ErrValidation{Message: "executable path required for a login item"}
	}
	abs, err := filepath.Abs(execPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
NoDisplay=true
X-GNOME-Autostart-enabled=true
`, desktopString(appID), desktopExecArg(abs))
	// Write then rename, so a crash never leaves a half-written entry.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// desktopString escapes s as a desktop entry string value.
func desktopString(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`, "\r", `\r`).Replace(s)
}

// desktopExecArg quotes one Exec argument: inside double quotes, `"`, "`",
// `$` and `\` are backslash-escaped, `%` is doubled (it starts a field
// code), and the result is escaped once more as a desktop entry string.
func desktopExecArg(s string) string {
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`).Replace(s) + `"`
	return desktopString(quoted)
}
