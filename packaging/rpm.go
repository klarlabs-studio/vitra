package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Overridable for tests.
var (
	lookPathRpmbuild = exec.LookPath
	runRpmbuild      = func(tool, topDir, specPath string) error {
		cmd := exec.Command(tool, "-bb", "--define", "_topdir "+topDir, specPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// ResolveRpmbuild returns the rpmbuild binary path (VITRA_RPMBUILD or PATH).
func ResolveRpmbuild() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_RPMBUILD")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_RPMBUILD=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathRpmbuild("rpmbuild")
	if err != nil {
		return "", fmt.Errorf("rpmbuild not found on PATH (set VITRA_RPMBUILD or install rpm-build): %w", err)
	}
	return p, nil
}

// BuildRPMDir stages an rpmbuild _topdir under outDir: payload/ (FreeDesktop
// tree), SPECS/<pkg>.spec, and empty BUILD/BUILDROOT/RPMS/SOURCES/SRPMS.
// Producing a final .rpm still requires rpmbuild (or VITRA_RPMBUILD).
func BuildRPMDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetLinuxRPM) {
		return Artifact{}, fmt.Errorf("BuildRPMDir requires target %s", TargetLinuxRPM)
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
	pkgName := debianName(spec.AppID, binName)
	arch := rpmArch(spec.Arch)
	ver, rel := rpmVersionRelease(spec.Version)

	for _, sub := range []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SRPMS", "SPECS", "payload"} {
		if err := os.MkdirAll(filepath.Join(outDir, sub), 0o755); err != nil {
			return Artifact{}, err
		}
	}

	payload := filepath.Join(outDir, "payload")
	usrBin := filepath.Join(payload, "usr", "bin")
	if err := os.MkdirAll(usrBin, 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(usrBin, binName), raw, 0o755); err != nil {
		return Artifact{}, err
	}

	fileList := []string{"/usr/bin/" + binName}
	iconKey := binName
	if strings.TrimSpace(spec.IconPath) != "" {
		stage, err := os.MkdirTemp("", "vitra-rpm-icon-*")
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
			pix := filepath.Join(payload, "usr", "share", "pixmaps")
			if err := os.MkdirAll(pix, 0o755); err != nil {
				return Artifact{}, err
			}
			if err := os.WriteFile(filepath.Join(pix, fileName), iconData, 0o644); err != nil {
				return Artifact{}, err
			}
			hicolorRel := hicolorIconRel(fileName)
			hicolorAbs := filepath.Join(payload, filepath.FromSlash(hicolorRel))
			if err := os.MkdirAll(filepath.Dir(hicolorAbs), 0o755); err != nil {
				return Artifact{}, err
			}
			if err := os.WriteFile(hicolorAbs, iconData, 0o644); err != nil {
				return Artifact{}, err
			}
			fileList = append(fileList, "/usr/share/pixmaps/"+fileName, "/"+hicolorRel)
		}
	}

	desktopRel := "usr/share/applications/" + sanitizeFileName(spec.AppID) + ".desktop"
	desktopBody := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=%s
Exec=/usr/bin/%s
Icon=%s
StartupWMClass=%s
Categories=%s
%sTerminal=false
`, spec.Name, spec.EffectiveDescription(), binName, iconKey, binName, spec.DesktopCategories(), spec.DesktopKeywordsLine())
	desktopAbs := filepath.Join(payload, filepath.FromSlash(desktopRel))
	if err := os.MkdirAll(filepath.Dir(desktopAbs), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(desktopAbs, []byte(desktopBody), 0o644); err != nil {
		return Artifact{}, err
	}
	fileList = append(fileList, "/"+desktopRel)

	metaRel := AppStreamMetainfoRel("usr/share", spec.AppID)
	if err := writeAppStreamMetainfo(spec, payload, metaRel); err != nil {
		return Artifact{}, err
	}
	fileList = append(fileList, "/"+metaRel)

	filesSection := strings.Join(fileList, "\n")
	changelogDate := time.Unix(0, 0).UTC().Format("Mon Jan 02 2006")
	urlLine := ""
	if home := strings.TrimSpace(spec.Homepage); home != "" {
		urlLine = "URL: " + home + "\n"
	}
	specBody := fmt.Sprintf(`Name: %s
Version: %s
Release: %s
Summary: %s
License: %s
Group: %s
%sPackager: %s
BuildArch: %s
AutoReqProv: no

%%description
%s

%%install
rm -rf %%{buildroot}
mkdir -p %%{buildroot}
cp -a %%{_topdir}/payload/. %%{buildroot}/

%%files
%s

%%changelog
* %s %s - %s-%s
- Packaged by Vitra
`, pkgName, ver, rel, spec.Name, spec.EffectiveLicense(), spec.RPMGroup(), urlLine, spec.EffectiveMaintainer(), arch,
		spec.EffectiveDescription(), filesSection, changelogDate,
		spec.EffectiveMaintainer(), ver, rel)

	specPath := filepath.Join(outDir, "SPECS", pkgName+".spec")
	if err := os.WriteFile(specPath, []byte(specBody), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxRPM,
		Path:   outDir,
		SHA256: digest,
		Signed: false,
	}, nil
}

// FoldRPM runs rpmbuild -bb against an rpmbuild _topdir and copies the
// produced .rpm to outPath.
func FoldRPM(topDir, outPath string) (Artifact, error) {
	if topDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("rpm topdir and output path are required")
	}
	info, err := os.Stat(topDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("rpm topdir must be a directory: %s", topDir)
	}
	specs, err := filepath.Glob(filepath.Join(topDir, "SPECS", "*.spec"))
	if err != nil {
		return Artifact{}, err
	}
	if len(specs) == 0 {
		return Artifact{}, fmt.Errorf("SPECS/*.spec missing in %s", topDir)
	}
	tool, err := ResolveRpmbuild()
	if err != nil {
		return Artifact{}, err
	}
	if err := runRpmbuild(tool, topDir, specs[0]); err != nil {
		return Artifact{}, fmt.Errorf("rpmbuild: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(topDir, "RPMS", "*", "*.rpm"))
	if err != nil {
		return Artifact{}, err
	}
	if len(matches) == 0 {
		return Artifact{}, fmt.Errorf("rpmbuild produced no .rpm under %s/RPMS", topDir)
	}
	src := matches[0]
	raw, err := os.ReadFile(src)
	if err != nil {
		return Artifact{}, fmt.Errorf("read rpm: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetLinuxRPM,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildRPM stages an rpmbuild tree then folds it into outPath (.rpm).
func BuildRPM(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output rpm path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-rpm-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if _, err := BuildRPMDir(spec, binaryPath, tmp); err != nil {
		return Artifact{}, err
	}
	return FoldRPM(tmp, outPath)
}

// rpmArch maps GOARCH / Spec.Arch to RPM BuildArch.
func rpmArch(arch string) string {
	if arch == "" {
		arch = DefaultArch()
	}
	switch arch {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	case "386", "i386", "i686":
		return "i686"
	default:
		return arch
	}
}

// rpmVersionRelease splits Spec.Version into RPM Version and Release.
// "1.2.3-beta" → Version=1.2.3 Release=beta; bare "1.2.3" → Release=1.
func rpmVersionRelease(version string) (ver, rel string) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "0", "1"
	}
	if i := strings.IndexByte(version, '-'); i >= 0 {
		ver = version[:i]
		rel = version[i+1:]
		if ver == "" {
			ver = "0"
		}
		if rel == "" {
			rel = "1"
		}
		return ver, rel
	}
	return version, "1"
}
