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
	lookPathHdiutil = exec.LookPath
	runDMGFold      = func(hdiutil, appPath, volName, outDMG string) error {
		cmd := exec.Command(hdiutil, "create",
			"-volname", volName,
			"-srcfolder", appPath,
			"-ov",
			"-format", "UDZO",
			outDMG,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// ResolveHdiutil returns the hdiutil binary path (VITRA_HDIUTIL or PATH).
func ResolveHdiutil() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_HDIUTIL")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_HDIUTIL=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathHdiutil("hdiutil")
	if err != nil {
		return "", fmt.Errorf("hdiutil not found on PATH (set VITRA_HDIUTIL or run on macOS): %w", err)
	}
	return p, nil
}

// FoldDMG runs hdiutil to produce a UDZO .dmg from a staged .app bundle.
// appPath may be the .app itself or a directory containing exactly one .app.
func FoldDMG(appPath, volName, outPath string) (Artifact, error) {
	if appPath == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("app path and output DMG path are required")
	}
	bundle, err := resolveAppBundle(appPath)
	if err != nil {
		return Artifact{}, err
	}
	if volName == "" {
		volName = strings.TrimSuffix(filepath.Base(bundle), ".app")
	}
	tool, err := ResolveHdiutil()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runDMGFold(tool, bundle, volName, outPath); err != nil {
		return Artifact{}, fmt.Errorf("hdiutil: %w", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read DMG: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetDarwinDMG,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

func resolveAppBundle(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("app path must be a directory: %s", path)
	}
	if strings.HasSuffix(strings.ToLower(path), ".app") {
		if _, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err != nil {
			return "", fmt.Errorf("info.plist missing in %s: %w", path, err)
		}
		return path, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var found string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".app") {
			if found != "" {
				return "", fmt.Errorf("multiple .app bundles in %s", path)
			}
			found = filepath.Join(path, e.Name())
		}
	}
	if found == "" {
		return "", fmt.Errorf("no .app bundle in %s", path)
	}
	if _, err := os.Stat(filepath.Join(found, "Contents", "Info.plist")); err != nil {
		return "", fmt.Errorf("info.plist missing in %s: %w", found, err)
	}
	return found, nil
}

// BuildDMG stages a Darwin .app then folds it into outPath (.dmg).
func BuildDMG(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output DMG path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-dmg-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	stageSpec := spec
	if !hasTarget(stageSpec, TargetDarwinApp) && !hasTarget(stageSpec, TargetDarwinDMG) {
		stageSpec.Targets = append(append([]Target(nil), stageSpec.Targets...), TargetDarwinDMG)
	}
	art, err := StageDarwinApp(stageSpec, binaryPath, tmp)
	if err != nil {
		return Artifact{}, err
	}
	return FoldDMG(art.Path, spec.Name, outPath)
}
