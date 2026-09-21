package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StageDarwinApp builds a macOS .app bundle under outDir:
//
//	<Name>.app/Contents/{Info.plist,MacOS/<exec>}
//
// If outDir already ends with ".app", that path is used as the bundle root.
// SigningIdentityRef must remain a reference (Spec.Validate); this stage never
// embeds secrets. Fold into a DMG with BuildDMG / FoldDMG (hdiutil).
func StageDarwinApp(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetDarwinApp) && !hasTarget(spec, TargetDarwinDMG) {
		return Artifact{}, fmt.Errorf("StageDarwinApp requires target %s or %s", TargetDarwinApp, TargetDarwinDMG)
	}
	if binaryPath == "" {
		return Artifact{}, fmt.Errorf("binary path is required")
	}
	src, err := os.Open(binaryPath)
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = src.Close() }()

	safeName := sanitizeFileName(spec.Name)
	bundlePath := outDir
	if !strings.HasSuffix(strings.ToLower(outDir), ".app") {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return Artifact{}, err
		}
		bundlePath = filepath.Join(outDir, safeName+".app")
	}
	macos := filepath.Join(bundlePath, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Join(bundlePath, "Contents", "Resources"), 0o755); err != nil {
		return Artifact{}, err
	}

	destPath := filepath.Join(macos, safeName)
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return Artifact{}, err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, h), src); err != nil {
		_ = dst.Close()
		return Artifact{}, err
	}
	if err := dst.Close(); err != nil {
		return Artifact{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	resources := filepath.Join(bundlePath, "Contents", "Resources")
	iconExtra := ""
	if _, fileName, err := stageIconFile(spec.IconPath, resources, safeName); err != nil {
		return Artifact{}, err
	} else if fileName != "" {
		// CFBundleIconFile: strip .icns (classic); keep other extensions.
		iconRef := fileName
		if strings.EqualFold(filepath.Ext(fileName), ".icns") {
			iconRef = strings.TrimSuffix(fileName, filepath.Ext(fileName))
		}
		iconExtra = fmt.Sprintf("\t<key>CFBundleIconFile</key>\n\t<string>%s</string>\n", xmlEscapeText(iconRef))
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>%s</string>
	<key>CFBundleVersion</key>
	<string>%s</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
%s</dict>
</plist>
`, xmlEscapeText(safeName), xmlEscapeText(spec.AppID), xmlEscapeText(spec.Name), xmlEscapeText(spec.Version), xmlEscapeText(spec.Version), iconExtra)
	if err := os.WriteFile(filepath.Join(bundlePath, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetDarwinApp,
		Path:   bundlePath,
		SHA256: sum,
		Signed: false,
	}, nil
}

func xmlEscapeText(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
