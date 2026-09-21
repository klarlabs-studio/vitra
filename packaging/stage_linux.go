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

// TargetLinuxDir is a staged Linux application directory (pre-AppImage/deb).
const TargetLinuxDir Target = "linux-dir"

// StageLinux copies binary into a FreeDesktop-style layout under outDir,
// writes a .desktop launcher, and returns an Artifact with the binary SHA-256.
// SigningIdentityRef must remain a reference (Spec.Validate); this stage never
// embeds secrets.
func StageLinux(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	hasDir := false
	for _, t := range spec.Targets {
		if t == TargetLinuxDir {
			hasDir = true
			break
		}
	}
	if !hasDir {
		return Artifact{}, fmt.Errorf("StageLinux requires target %s", TargetLinuxDir)
	}
	if binaryPath == "" {
		return Artifact{}, fmt.Errorf("binary path is required")
	}
	src, err := os.Open(binaryPath)
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = src.Close() }()

	binDir := filepath.Join(outDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return Artifact{}, err
	}
	safeName := sanitizeFileName(spec.Name)
	destPath := filepath.Join(binDir, safeName)
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

	iconKey := safeName
	if _, _, err := stageIconFile(spec.IconPath, outDir, safeName); err != nil {
		return Artifact{}, err
	}

	desktop := filepath.Join(outDir, sanitizeFileName(spec.AppID)+".desktop")
	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
Icon=%s
Categories=Utility;
StartupNotify=true
`, spec.Name, destPath, iconKey)
	if err := os.WriteFile(desktop, []byte(body), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxDir,
		Path:   outDir,
		SHA256: sum,
		Signed: false,
	}, nil
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "app"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
}
