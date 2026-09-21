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
	lookPathMakensis = exec.LookPath
	runMakensis      = func(tool, nsisDir, outPath string) error {
		nsi := filepath.Join(nsisDir, "installer.nsi")
		cmd := exec.Command(tool, "-V2", "-XOutFile "+outPath, nsi)
		cmd.Dir = nsisDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// ResolveMakensis returns the makensis binary path (VITRA_MAKENSIS or PATH).
func ResolveMakensis() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_MAKENSIS")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_MAKENSIS=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathMakensis("makensis")
	if err != nil {
		return "", fmt.Errorf("makensis not found on PATH (set VITRA_MAKENSIS or install NSIS): %w", err)
	}
	return p, nil
}

// FoldNSIS runs makensis to produce a final setup .exe from an NSIS stage directory.
func FoldNSIS(nsisDir, outPath string) (Artifact, error) {
	if nsisDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("nsis dir and output path are required")
	}
	info, err := os.Stat(nsisDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("nsis dir must be a directory: %s", nsisDir)
	}
	if _, err := os.Stat(filepath.Join(nsisDir, "installer.nsi")); err != nil {
		return Artifact{}, fmt.Errorf("installer.nsi missing in %s: %w", nsisDir, err)
	}
	tool, err := ResolveMakensis()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runMakensis(tool, nsisDir, outPath); err != nil {
		return Artifact{}, fmt.Errorf("makensis: %w", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read NSIS installer: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetWindowsNSIS,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildNSIS stages an NSIS directory then folds it into outPath (setup .exe).
func BuildNSIS(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output NSIS installer path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-nsis-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if _, err := BuildNSISDir(spec, binaryPath, tmp); err != nil {
		return Artifact{}, err
	}
	return FoldNSIS(tmp, outPath)
}

// BuildNSISDir stages a Windows payload and writes installer.nsi for NSIS.
// The script installs under $LOCALAPPDATA, writes Uninstall.exe, registers
// Add/Remove Programs keys under HKCU, and creates Start Menu + Desktop
// shortcuts. Producing a final setup .exe still requires makensis (or
// VITRA_MAKENSIS); this layout is the portable intermediate that tool consumes.
func BuildNSISDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetWindowsNSIS) && !hasTarget(spec, TargetWindowsDir) {
		return Artifact{}, fmt.Errorf("BuildNSISDir requires target %s or %s", TargetWindowsNSIS, TargetWindowsDir)
	}
	art, err := StageWindows(withWindowsTargets(spec, TargetWindowsNSIS), binaryPath, outDir)
	if err != nil {
		return Artifact{}, err
	}
	safeName := sanitizeFileName(spec.Name)
	exeName := safeName
	if !strings.HasSuffix(strings.ToLower(exeName), ".exe") {
		exeName += ".exe"
	}
	outInstaller := safeName + "-setup.exe"
	iconFile := ""
	iconDefine := ""
	iconFileLine := ""
	startMenuShortcut := `  CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"`
	desktopShortcut := `  CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"`
	iconUninstall := ""
	displayIconReg := ""
	if spec.IconPath != "" {
		base := strings.TrimSuffix(exeName, filepath.Ext(exeName))
		ext := strings.ToLower(filepath.Ext(spec.IconPath))
		if ext == "" {
			ext = ".ico"
		}
		iconFile = base + ext
		iconDefine = fmt.Sprintf("!define PRODUCT_ICON \"%s\"\n", iconFile)
		iconFileLine = "  File \"bin\\${PRODUCT_ICON}\"\n"
		startMenuShortcut = `  CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}" 0`
		desktopShortcut = `  CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}" 0`
		iconUninstall = "  Delete \"$INSTDIR\\${PRODUCT_ICON}\"\n"
		displayIconReg = "  WriteRegStr HKCU \"${UNINST_KEY}\" \"DisplayIcon\" \"$INSTDIR\\${PRODUCT_ICON}\"\n"
	}
	shortcuts := startMenuShortcut + "\n" + desktopShortcut + "\n"
	homepageReg := ""
	if home := strings.TrimSpace(spec.Homepage); home != "" {
		homepageReg = fmt.Sprintf("  WriteRegStr HKCU \"${UNINST_KEY}\" \"URLInfoAbout\" \"%s\"\n", nsisEscape(home))
	}
	commentsReg := fmt.Sprintf("  WriteRegStr HKCU \"${UNINST_KEY}\" \"Comments\" \"%s\"\n", nsisEscape(spec.EffectiveDescription()))
	copyrightReg := fmt.Sprintf("  WriteRegStr HKCU \"${UNINST_KEY}\" \"LegalCopyright\" \"%s\"\n", nsisEscape(spec.EffectiveLicense()))
	nsi := fmt.Sprintf(`; Generated by vitra package — fold with makensis (or VITRA_MAKENSIS).
!define PRODUCT_NAME "%s"
!define PRODUCT_VERSION "%s"
!define PRODUCT_PUBLISHER "%s"
!define PRODUCT_APPID "%s"
!define PRODUCT_EXE "%s"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_APPID}"
%s
Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "%s"
InstallDir "$LOCALAPPDATA\${PRODUCT_NAME}"
RequestExecutionLevel user
SetCompressor /SOLID lzma

Section "MainSection" SEC01
  SetOutPath "$INSTDIR"
  File "bin\${PRODUCT_EXE}"
%s%s  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
%s%s%s%sSectionEnd

Section "Uninstall"
  Delete "$INSTDIR\${PRODUCT_EXE}"
  Delete "$INSTDIR\Uninstall.exe"
%s  Delete "$SMPROGRAMS\${PRODUCT_NAME}.lnk"
  Delete "$DESKTOP\${PRODUCT_NAME}.lnk"
  DeleteRegKey HKCU "${UNINST_KEY}"
  RMDir "$INSTDIR"
SectionEnd
`, nsisEscape(spec.Name), nsisEscape(spec.Version), nsisEscape(spec.EffectivePublisher()),
		nsisEscape(spec.AppID), exeName, iconDefine, outInstaller, iconFileLine, shortcuts,
		displayIconReg, homepageReg, commentsReg, copyrightReg, iconUninstall)
	if err := os.WriteFile(filepath.Join(outDir, "installer.nsi"), []byte(nsi), 0o644); err != nil {
		return Artifact{}, err
	}
	art.Target = TargetWindowsNSIS
	return art, nil
}

func nsisEscape(s string) string {
	return strings.ReplaceAll(s, `"`, `'`)
}
