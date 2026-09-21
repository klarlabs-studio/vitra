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
	lookPathSnapcraft = exec.LookPath
	runSnapcraftPack  = func(tool, primeDir, outPath string) error {
		cmd := exec.Command(tool, "pack", primeDir, "--output", outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// ResolveSnapcraft returns the snapcraft binary path (VITRA_SNAPCRAFT or PATH).
func ResolveSnapcraft() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_SNAPCRAFT")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_SNAPCRAFT=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathSnapcraft("snapcraft")
	if err != nil {
		return "", fmt.Errorf("snapcraft not found on PATH (set VITRA_SNAPCRAFT or install snapcraft): %w", err)
	}
	return p, nil
}

// BuildSnapDir stages a snap prime directory under outDir: FreeDesktop payload
// (usr/bin, .desktop, icons) plus meta/snap.yaml. Producing a final .snap still
// requires snapcraft pack (or VITRA_SNAPCRAFT).
func BuildSnapDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetLinuxSnap) {
		return Artifact{}, fmt.Errorf("BuildSnapDir requires target %s", TargetLinuxSnap)
	}
	if binaryPath == "" {
		return Artifact{}, fmt.Errorf("binary path is required")
	}
	if outDir == "" {
		return Artifact{}, fmt.Errorf("output directory is required")
	}
	raw, err := os.ReadFile(binaryPath)
	if err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	binName := sanitizeFileName(spec.Name)
	snapName := snapPackageName(spec.AppID, binName)
	arch := snapArch(spec.Arch)

	usrBin := filepath.Join(outDir, "usr", "bin")
	if err := os.MkdirAll(usrBin, 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(usrBin, binName), raw, 0o755); err != nil {
		return Artifact{}, err
	}

	iconKey := binName
	if _, _, err := stageFreedesktopIcons(spec.IconPath, outDir, binName, false); err != nil {
		return Artifact{}, err
	}

	desktopRel := "usr/share/applications/" + sanitizeFileName(spec.AppID) + ".desktop"
	desktopBody := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=%s
Exec=%s
Icon=%s
StartupWMClass=%s
Categories=Utility;
Terminal=false
`, spec.Name, spec.EffectiveDescription(), binName, iconKey, binName)
	desktopAbs := filepath.Join(outDir, filepath.FromSlash(desktopRel))
	if err := os.MkdirAll(filepath.Dir(desktopAbs), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(desktopAbs, []byte(desktopBody), 0o644); err != nil {
		return Artifact{}, err
	}
	if err := writeAppStreamMetainfo(spec, outDir, AppStreamMetainfoRel("usr/share", spec.AppID)); err != nil {
		return Artifact{}, err
	}

	metaDir := filepath.Join(outDir, "meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return Artifact{}, err
	}
	desc := indentYAMLBlock(spec.EffectiveDescription())
	yaml := fmt.Sprintf(`name: %s
version: %q
summary: %s
description: |
%s
architectures:
  - %s
base: core22
confinement: strict
grade: stable
apps:
  %s:
    command: usr/bin/%s
    desktop: %s
`, snapName, spec.Version, yamlScalar(spec.Name), desc, arch, snapName, binName, desktopRel)
	if err := os.WriteFile(filepath.Join(metaDir, "snap.yaml"), []byte(yaml), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxSnap,
		Path:   outDir,
		SHA256: digest,
		Signed: false,
	}, nil
}

// FoldSnap runs snapcraft pack against a prime directory and writes outPath.
func FoldSnap(primeDir, outPath string) (Artifact, error) {
	if primeDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("snap prime dir and output path are required")
	}
	info, err := os.Stat(primeDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("snap prime dir must be a directory: %s", primeDir)
	}
	if _, err := os.Stat(filepath.Join(primeDir, "meta", "snap.yaml")); err != nil {
		return Artifact{}, fmt.Errorf("meta/snap.yaml missing in %s: %w", primeDir, err)
	}
	tool, err := ResolveSnapcraft()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runSnapcraftPack(tool, primeDir, outPath); err != nil {
		return Artifact{}, fmt.Errorf("snapcraft pack: %w", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read snap: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetLinuxSnap,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildSnap stages a snap prime directory then folds it into outPath (.snap).
func BuildSnap(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output snap path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-snap-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if _, err := BuildSnapDir(spec, binaryPath, tmp); err != nil {
		return Artifact{}, err
	}
	return FoldSnap(tmp, outPath)
}

func snapPackageName(appID, fallback string) string {
	s := debianName(appID, fallback)
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, s)
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if s == "" {
		s = "vitra-app"
	}
	if len(s) > 40 {
		s = s[:40]
		s = strings.Trim(s, "-")
	}
	return s
}

func snapArch(arch string) string {
	if arch == "" {
		arch = DefaultArch()
	}
	switch arch {
	case "amd64", "x86_64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	case "arm", "armhf":
		return "armhf"
	case "386", "i386":
		return "i386"
	default:
		return arch
	}
}

func yamlScalar(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|>!%@`'") || strings.Contains(s, "\n") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func indentYAMLBlock(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = DefaultDescription
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
