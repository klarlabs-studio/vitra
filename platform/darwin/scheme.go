package darwin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var schemeName = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

// supportHome returns the directory for helper .app bundles.
// Tests replace this with a temp dir.
var supportHome = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "vitra"), nil
}

// lsregisterRunner best-effort registers a bundle with Launch Services.
var lsregisterRunner = func(appPath string) {
	candidates := []string{
		"/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
		"/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister",
	}
	for _, bin := range candidates {
		if _, err := os.Stat(bin); err != nil {
			continue
		}
		_ = exec.Command(bin, "-f", appPath).Run()
		return
	}
}

func safeBundleName(appID string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, appID)
	return safe
}

func writeHelperApp(bundleDir, bundleID, name, execPath, plistExtra string) error {
	macos := filepath.Join(bundleDir, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		return err
	}
	launcher := filepath.Join(macos, "launcher")
	script := "#!/bin/sh\nexec " + shellQuote(execPath) + " \"$@\"\n"
	if err := os.WriteFile(launcher, []byte(script), 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleExecutable</key>
	<string>launcher</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
%s</dict>
</plist>
`, xmlEscape(bundleID), xmlEscape(name), plistExtra)
	return os.WriteFile(filepath.Join(bundleDir, "Contents", "Info.plist"), []byte(plist), 0o644)
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

// RegisterURLScheme installs a user-local helper .app with CFBundleURLTypes so
// scheme://… can launch execPath. Best-effort lsregister. Does not require WKWebView.
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
	bundle := filepath.Join(root, safeBundleName(appID)+"-url.app")
	_ = os.RemoveAll(bundle)
	extra := fmt.Sprintf(`	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLName</key>
			<string>%s</string>
			<key>CFBundleURLSchemes</key>
			<array>
				<string>%s</string>
			</array>
		</dict>
	</array>
`, xmlEscape(appID), xmlEscape(scheme))
	if err := writeHelperApp(bundle, appID+".url", appID, abs, extra); err != nil {
		return err
	}
	lsregisterRunner(bundle)
	return nil
}

// UnregisterURLScheme removes the helper .app for appID URL handling.
func (h *Host) UnregisterURLScheme(appID string) error {
	if appID == "" {
		return fmt.Errorf("app id required")
	}
	root, err := supportHome()
	if err != nil {
		return err
	}
	path := filepath.Join(root, safeBundleName(appID)+"-url.app")
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
