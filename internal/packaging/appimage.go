package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Overridable for tests.
var (
	lookPath        = exec.LookPath
	runAppImageTool = func(tool, appDir, outPath string) error {
		cmd := exec.Command(tool, appDir, outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "ARCH="+DefaultArch())
		return cmd.Run()
	}
)

// ResolveAppImageTool returns the appimagetool binary path.
// Honors VITRA_APPIMAGETOOL, otherwise PATH.
func ResolveAppImageTool() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_APPIMAGETOOL")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_APPIMAGETOOL=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPath("appimagetool")
	if err != nil {
		return "", fmt.Errorf("appimagetool not found on PATH (set VITRA_APPIMAGETOOL or install appimagetool): %w", err)
	}
	return p, nil
}

// FoldAppDir runs appimagetool to produce a final .AppImage from an AppDir.
func FoldAppDir(appDir, outPath string) (Artifact, error) {
	if appDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("appdir and output path are required")
	}
	info, err := os.Stat(appDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("appdir must be a directory: %s", appDir)
	}
	tool, err := ResolveAppImageTool()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runAppImageTool(tool, appDir, outPath); err != nil {
		return Artifact{}, fmt.Errorf("appimagetool: %w", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read AppImage: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetLinuxAppImage,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildAppImage stages an AppDir then folds it into outPath (.AppImage).
func BuildAppImage(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output AppImage path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-appdir-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	appDir := filepath.Join(tmp, sanitizeFileName(spec.Name)+".AppDir")
	if _, err := BuildAppDir(spec, binaryPath, appDir); err != nil {
		return Artifact{}, err
	}
	return FoldAppDir(appDir, outPath)
}
