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
	lookPathCandle = exec.LookPath
	lookPathLight  = exec.LookPath
	runWiXFold     = func(candle, light, wixDir, outMSI string) error {
		wxs := filepath.Join(wixDir, "product.wxs")
		obj := filepath.Join(wixDir, "product.wixobj")
		cmd := exec.Command(candle, "-nologo", "-out", obj, wxs)
		cmd.Dir = wixDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("candle: %w", err)
		}
		cmd = exec.Command(light, "-nologo", "-out", outMSI, obj)
		cmd.Dir = wixDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("light: %w", err)
		}
		return nil
	}
)

// ResolveCandle returns the WiX candle binary path (VITRA_CANDLE or PATH).
func ResolveCandle() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_CANDLE")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_CANDLE=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathCandle("candle")
	if err != nil {
		return "", fmt.Errorf("candle not found on PATH (set VITRA_CANDLE or install WiX Toolset): %w", err)
	}
	return p, nil
}

// ResolveLight returns the WiX light binary path (VITRA_LIGHT or PATH).
func ResolveLight() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VITRA_LIGHT")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VITRA_LIGHT=%s: %w", p, err)
		}
		return p, nil
	}
	p, err := lookPathLight("light")
	if err != nil {
		return "", fmt.Errorf("light not found on PATH (set VITRA_LIGHT or install WiX Toolset): %w", err)
	}
	return p, nil
}

// FoldMSI runs candle+light to produce a final .msi from a WiX stage directory.
func FoldMSI(wixDir, outPath string) (Artifact, error) {
	if wixDir == "" || outPath == "" {
		return Artifact{}, fmt.Errorf("wix dir and output path are required")
	}
	info, err := os.Stat(wixDir)
	if err != nil {
		return Artifact{}, err
	}
	if !info.IsDir() {
		return Artifact{}, fmt.Errorf("wix dir must be a directory: %s", wixDir)
	}
	if _, err := os.Stat(filepath.Join(wixDir, "product.wxs")); err != nil {
		return Artifact{}, fmt.Errorf("product.wxs missing in %s: %w", wixDir, err)
	}
	candle, err := ResolveCandle()
	if err != nil {
		return Artifact{}, err
	}
	light, err := ResolveLight()
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if err := runWiXFold(candle, light, wixDir, outPath); err != nil {
		return Artifact{}, err
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read MSI: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Artifact{
		Target: TargetWindowsMSI,
		Path:   outPath,
		SHA256: hex.EncodeToString(sum[:]),
		Signed: false,
	}, nil
}

// BuildMSI stages a WiX directory then folds it into outPath (.msi).
func BuildMSI(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if outPath == "" {
		return Artifact{}, fmt.Errorf("output MSI path is required")
	}
	tmp, err := os.MkdirTemp("", "vitra-wix-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if _, err := BuildWiXDir(spec, binaryPath, tmp); err != nil {
		return Artifact{}, err
	}
	return FoldMSI(tmp, outPath)
}

// BuildWiXDir stages a Windows payload and writes product.wxs for WiX Toolset.
// The script installs under LocalAppDataFolder, emits Start Menu + Desktop
// shortcuts, and uses a deterministic UpgradeCode GUID derived from AppID.
// Producing a final .msi still requires candle/light (or VITRA_CANDLE /
// VITRA_LIGHT); this layout is the portable intermediate those tools consume.
func BuildWiXDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetWindowsMSI) && !hasTarget(spec, TargetWindowsDir) {
		return Artifact{}, fmt.Errorf("BuildWiXDir requires target %s or %s", TargetWindowsMSI, TargetWindowsDir)
	}
	art, err := StageWindows(withWindowsTargets(spec, TargetWindowsMSI), binaryPath, outDir)
	if err != nil {
		return Artifact{}, err
	}
	safeName := sanitizeFileName(spec.Name)
	exeName := safeName
	if !strings.HasSuffix(strings.ToLower(exeName), ".exe") {
		exeName += ".exe"
	}
	arch := spec.Arch
	if arch == "" {
		arch = DefaultArch()
	}
	iconFile := ""
	if spec.IconPath != "" {
		base := strings.TrimSuffix(exeName, filepath.Ext(exeName))
		ext := strings.ToLower(filepath.Ext(spec.IconPath))
		if ext == "" {
			ext = ".ico"
		}
		iconFile = base + ext
	}
	iconXML := ""
	iconComp := ""
	shortcutIconAttr := ""
	if iconFile != "" {
		iconID := "AppIconFile"
		iconXML = fmt.Sprintf(`
    <Icon Id="AppIcon" SourceFile="bin\%s"/>
    <Property Id="ARPPRODUCTICON" Value="AppIcon"/>`, iconFile)
		iconComp = fmt.Sprintf(`
      <Component Id="AppIconComponent" Guid="*">
        <File Id="%s" Source="bin\%s" KeyPath="yes"/>
      </Component>`, iconID, iconFile)
		shortcutIconAttr = `
                  Icon="AppIcon"`
	}
	homepageXML := ""
	if home := strings.TrimSpace(spec.Homepage); home != "" {
		homepageXML = fmt.Sprintf(`
    <Property Id="ARPURLINFOABOUT" Value="%s"/>`, xmlEscape(home))
	}
	desc := xmlEscape(spec.EffectiveDescription())
	commentsXML := fmt.Sprintf(`
    <Property Id="ARPCOMMENTS" Value="%s"/>`, desc)
	aumidProp := fmt.Sprintf(`
                  <ShortcutProperty Key="System.AppUserModel.ID" Value="%s"/>`, xmlEscape(spec.AppID))
	upgrade := deterministicGUID("vitra-wix-upgrade:" + spec.AppID)
	regManufacturer := xmlEscape(safeName)
	regProduct := xmlEscape(safeName)
	wxs := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="%s" Language="1033" Version="%s"
           Manufacturer="%s" UpgradeCode="{%s}">
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perUser"
             Description="%s" Comments="%s"/>
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed."/>
    <MediaTemplate EmbedCab="yes"/>%s%s%s
    <Feature Id="ProductFeature" Title="%s" Level="1">
      <ComponentGroupRef Id="ProductComponents"/>
    </Feature>
  </Product>
  <Fragment>
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="LocalAppDataFolder">
        <Directory Id="INSTALLFOLDER" Name="%s"/>
      </Directory>
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="%s"/>
      </Directory>
      <Directory Id="DesktopFolder" Name="Desktop"/>
    </Directory>
  </Fragment>
  <Fragment>
    <ComponentGroup Id="ProductComponents" Directory="INSTALLFOLDER">
      <Component Id="MainExecutable" Guid="*">
        <File Id="AppExe" Source="bin\%s" KeyPath="yes" Checksum="yes"/>
      </Component>%s
      <Component Id="StartMenuShortcut" Guid="*" Directory="ApplicationProgramsFolder">
        <Shortcut Id="AppStartMenuShortcut" Name="%s"
                  Description="%s"
                  Target="[INSTALLFOLDER]%s"
                  WorkingDirectory="INSTALLFOLDER"%s>%s
        </Shortcut>
        <RemoveFolder Id="RemoveAppProgramsFolder" Directory="ApplicationProgramsFolder" On="uninstall"/>
        <RegistryValue Root="HKCU" Key="Software\%s\%s" Name="StartMenuShortcut" Type="integer" Value="1" KeyPath="yes"/>
      </Component>
      <Component Id="DesktopShortcut" Guid="*" Directory="DesktopFolder">
        <Shortcut Id="AppDesktopShortcut" Name="%s"
                  Description="%s"
                  Target="[INSTALLFOLDER]%s"
                  WorkingDirectory="INSTALLFOLDER"%s>%s
        </Shortcut>
        <RegistryValue Root="HKCU" Key="Software\%s\%s" Name="DesktopShortcut" Type="integer" Value="1" KeyPath="yes"/>
      </Component>
    </ComponentGroup>
  </Fragment>
</Wix>
`, xmlEscape(spec.Name), xmlEscape(spec.Version), xmlEscape(spec.EffectivePublisher()), upgrade,
		desc, desc, commentsXML, homepageXML, iconXML, xmlEscape(spec.Name), xmlEscape(safeName), xmlEscape(safeName),
		exeName, iconComp,
		xmlEscape(spec.Name), xmlEscape(spec.Name), exeName, shortcutIconAttr, aumidProp, regManufacturer, regProduct,
		xmlEscape(spec.Name), xmlEscape(spec.Name), exeName, shortcutIconAttr, aumidProp, regManufacturer, regProduct)
	_ = arch // recorded in Spec / provenance; WiX Platform can be set at candle time
	if err := os.WriteFile(filepath.Join(outDir, "product.wxs"), []byte(wxs), 0o644); err != nil {
		return Artifact{}, err
	}
	art.Target = TargetWindowsMSI
	return art, nil
}

// deterministicGUID returns a stable UUID string (no braces) derived from seed.
func deterministicGUID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

func withWindowsTargets(spec Spec, prefer Target) Spec {
	out := spec
	out.Targets = []Target{prefer, TargetWindowsDir}
	return out
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&quot;`,
		`'`, `&apos;`,
	)
	return r.Replace(s)
}
