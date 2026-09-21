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

// TargetWindowsDir is a staged Windows application directory (pre-MSI/NSIS).
const TargetWindowsDir Target = "windows-dir"

// StageWindows copies the input binary into a Windows-oriented layout under
// outDir as bin/<Name>.exe and returns an Artifact with the binary SHA-256.
// SigningIdentityRef must remain a reference (Spec.Validate); this stage never
// embeds secrets.
func StageWindows(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetWindowsDir) && !hasTarget(spec, TargetWindowsMSI) && !hasTarget(spec, TargetWindowsNSIS) {
		return Artifact{}, fmt.Errorf("StageWindows requires target %s, %s, or %s", TargetWindowsDir, TargetWindowsMSI, TargetWindowsNSIS)
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
	if !strings.HasSuffix(strings.ToLower(safeName), ".exe") {
		safeName += ".exe"
	}
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

	base := strings.TrimSuffix(safeName, filepath.Ext(safeName))
	if base == "" {
		base = "app"
	}
	if _, _, err := stageIconFile(spec.IconPath, binDir, base); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetWindowsDir,
		Path:   outDir,
		SHA256: sum,
		Signed: false,
	}, nil
}
