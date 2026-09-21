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
	lookPathFlatpakBuilder = exec.LookPath
	runFlatpakPack         = func(tool, stageDir, outPath string) error {
		cmd := exec.Command(tool, stageDir, outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// ResolveFlatpakBuilder returns a Flatpak fold tool path.
// Honors VITRA_FLATPAK_BUILDER (wrapper or fake: <stage> <out.flatpak>),
// otherwise PATH lookup for flatpak-builder.
func ResolveFlatpakBuilder() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_FLATPAK_BUILDER")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_FLATPAK_BUILDER=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathFlatpakBuilder("flatpak-builder")
	if err != nil {
		return "", fmt.Errorf("flatpak-builder not found on PATH (set VITRA_FLATPAK_BUILDER or install flatpak-builder): %w", err)
	}
	return p, nil
}

// BuildFlatpakDir stages a Flatpak application tree under outDir:
// files/{bin,share/...} (maps to /app), metadata, and a flatpak-builder
// manifest.yml. Producing a final .flatpak still requires a fold tool
// (VITRA_FLATPAK_BUILDER wrapper or flatpak-builder).
func BuildFlatpakDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetLinuxFlatpak) {
		return Artifact{}, fmt.Errorf("BuildFlatpakDir requires target %s", TargetLinuxFlatpak)
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
	appID := flatpakAppID(spec.AppID)
	arch := flatpakArch(spec.Arch)

	filesRoot := filepath.Join(outDir, "files")
	binDir := filepath.Join(filesRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(binDir, binName), raw, 0o755); err != nil {
		return Artifact{}, err
	}

	iconKey := binName
	if strings.TrimSpace(spec.IconPath) != "" {
		stage, err := os.MkdirTemp("", "vitra-flatpak-icon-*")
		if err != nil {
			return Artifact{}, err
		}
		defer func() { _ = os.RemoveAll(stage) }()
		_, fileName, err := stageIconFile(spec.IconPath, stage, binName)
		if err != nil {
			return Artifact{}, err
		}
		if fileName != "" {
			iconData, err := os.ReadFile(filepath.Join(stage, fileName))
			if err != nil {
				return Artifact{}, err
			}
			// Flatpak hicolor under /app/share/icons/...
			hicolorRel := "share/icons/hicolor/"
			ext := strings.ToLower(filepath.Ext(fileName))
			if ext == ".svg" {
				hicolorRel += "scalable/apps/" + fileName
			} else {
				hicolorRel += "256x256/apps/" + fileName
			}
			abs := filepath.Join(filesRoot, filepath.FromSlash(hicolorRel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return Artifact{}, err
			}
			if err := os.WriteFile(abs, iconData, 0o644); err != nil {
				return Artifact{}, err
			}
		}
	}

	desktopRel := "share/applications/" + appID + ".desktop"
	desktopBody := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=%s
Exec=%s
Icon=%s
StartupWMClass=%s
Categories=%s
%sTerminal=false
X-GNOME-UsesNotifications=true
`, spec.Name, spec.EffectiveDescription(), binName, iconKey, binName, spec.DesktopCategories(), spec.DesktopKeywordsLine())
	desktopAbs := filepath.Join(filesRoot, filepath.FromSlash(desktopRel))
	if err := os.MkdirAll(filepath.Dir(desktopAbs), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(desktopAbs, []byte(desktopBody), 0o644); err != nil {
		return Artifact{}, err
	}

	metaRel := AppStreamMetainfoRel("share", appID)
	if err := writeAppStreamMetainfo(spec, filesRoot, metaRel); err != nil {
		return Artifact{}, err
	}

	meta := fmt.Sprintf(`[Application]
name=%s
runtime=org.freedesktop.Platform/%s/23.08
sdk=org.freedesktop.Sdk/%s/23.08
command=%s

%s
`, appID, arch, arch, binName, flatpakMetadataContext())
	if err := os.WriteFile(filepath.Join(outDir, "metadata"), []byte(meta), 0o644); err != nil {
		return Artifact{}, err
	}

	manifest := fmt.Sprintf(`app-id: %s
runtime: org.freedesktop.Platform
runtime-version: "23.08"
sdk: org.freedesktop.Sdk
command: %s
finish-args:
%s
modules:
  - name: app
    buildsystem: simple
    build-commands:
      - cp -a files/. /app/
    sources:
      - type: dir
        path: .
`, appID, binName, flatpakPortalFinishArgsYAML())
	if err := os.WriteFile(filepath.Join(outDir, "manifest.yml"), []byte(manifest), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxFlatpak,
		Path:   outDir,
		SHA256: digest,
		Signed: false,
	}, nil
}

// flatpakPortalFinishArgsYAML is the default portal-oriented finish-args block
// for desktop Vitra apps: display + GPU, network for updates, and talk to
// xdg-desktop-portal instead of a broad --filesystem=home grant.
func flatpakPortalFinishArgsYAML() string {
	args := []string{
		"--share=ipc",
		"--share=network",
		"--socket=fallback-x11",
		"--socket=wayland",
		"--device=dri",
		"--talk-name=org.freedesktop.portal.Desktop",
		"--talk-name=org.freedesktop.portal.Documents",
		"--talk-name=org.freedesktop.portal.FileChooser",
		"--talk-name=org.freedesktop.portal.OpenURI",
		"--talk-name=org.freedesktop.Notifications",
	}
	var b strings.Builder
	for _, a := range args {
		b.WriteString("  - ")
		b.WriteString(a)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// flatpakMetadataContext mirrors finish-args into Flatpak metadata Context /
// Session Bus Policy sections (stage tree used by fold wrappers).
func flatpakMetadataContext() string {
	return `[Context]
shared=network;ipc;
sockets=x11;wayland;fallback-x11;
devices=dri;

[Session Bus Policy]
org.freedesktop.portal.Desktop=talk
org.freedesktop.portal.Documents=talk
org.freedesktop.portal.FileChooser=talk
org.freedesktop.portal.OpenURI=talk
org.freedesktop.Notifications=talk`
}

// FoldFlatpak runs the Flatpak fold tool against a staged directory and writes
// outPath (.flatpak). The tool is invoked as: <tool> <stageDir> <outPath>
// (set VITRA_FLATPAK_BUILDER to a wrapper that runs flatpak-builder +
// build-bundle, or to a CI fake).
func FoldFlatpak(stageDir, outPath string) (Artifact, error) {
	if stageDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("flatpak stage dir and output path are required")
	}
	info, err := os.Stat(stageDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("flatpak stage dir must be a directory: %s", stageDir)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "metadata")); err != nil {
		return Artifact{}, fmt.Errorf("metadata missing in %s: %w", stageDir, err)
	}
	tool, err := ResolveFlatpakBuilder()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runFlatpakPack(tool, stageDir, outPath); err != nil {
		return Artifact{}, fmt.Errorf("flatpak fold: %w", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read flatpak: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetLinuxFlatpak,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildFlatpak stages a Flatpak tree then folds it into outPath (.flatpak).
func BuildFlatpak(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output flatpak path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-flatpak-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if _, err := BuildFlatpakDir(spec, binaryPath, tmp); err != nil {
		return Artifact{}, err
	}
	return FoldFlatpak(tmp, outPath)
}

func flatpakAppID(appID string) string {
	s := strings.TrimSpace(appID)
	if s == "" {
		return "com.vitra.app"
	}
	// Flatpak app-ids are reverse-DNS with [A-Za-z0-9._-], segments starting with a letter.
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		default:
			return '.'
		}
	}, s)
	for strings.Contains(s, "..") {
		s = strings.ReplaceAll(s, "..", ".")
	}
	return strings.Trim(s, ".")
}

func flatpakArch(arch string) string {
	if arch == "" {
		arch = DefaultArch()
	}
	switch arch {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	case "386", "i386", "i686":
		return "i386"
	default:
		return arch
	}
}
