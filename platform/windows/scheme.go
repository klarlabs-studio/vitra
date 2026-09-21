package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var schemeName = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

// supportHome returns the directory for helper registration files.
var supportHome = func() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(dir, "vitra"), nil
}

// registryImportRunner best-effort imports a .reg file. Tests replace this.
var registryImportRunner = func(regPath string) {
	if _, err := exec.LookPath("reg"); err != nil {
		return
	}
	_ = exec.Command("reg", "import", regPath).Run()
}

func safeName(appID string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, appID)
}

func regEscape(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}

// RegisterURLScheme writes an HKCU Classes .reg helper for scheme and
// best-effort imports it. Does not require the native WebView2 host.
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
	root, err := supportHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	regPath := filepath.Join(root, safeName(appID)+"-url.reg")
	cmd := fmt.Sprintf("\"%s\" \"%%1\"", strings.ReplaceAll(abs, `"`, `\"`))
	body := fmt.Sprintf(`Windows Registry Editor Version 5.00

[HKEY_CURRENT_USER\Software\Classes\%s]
@="URL:%s Protocol"
"URL Protocol"=""

[HKEY_CURRENT_USER\Software\Classes\%s\shell\open\command]
@="%s"
`, scheme, scheme, scheme, regEscape(cmd))
	if err := os.WriteFile(regPath, []byte(body), 0o644); err != nil {
		return err
	}
	registryImportRunner(regPath)
	return nil
}

// UnregisterURLScheme removes the helper .reg file for appID.
func (h *Host) UnregisterURLScheme(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	root, err := supportHome()
	if err != nil {
		return err
	}
	path := filepath.Join(root, safeName(appID)+"-url.reg")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
