package packaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildWiXDir stages a Windows payload and writes product.wxs for WiX Toolset.
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
	wxs := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="%s" Language="1033" Version="%s"
           Manufacturer="%s" UpgradeCode="%s">
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perUser"
             Description="%s" Comments="Vitra-packaged Windows installer"/>
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed."/>
    <MediaTemplate EmbedCab="yes"/>
    <Feature Id="ProductFeature" Title="%s" Level="1">
      <ComponentGroupRef Id="ProductComponents"/>
    </Feature>
  </Product>
  <Fragment>
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="LocalAppDataFolder">
        <Directory Id="INSTALLFOLDER" Name="%s"/>
      </Directory>
    </Directory>
  </Fragment>
  <Fragment>
    <ComponentGroup Id="ProductComponents" Directory="INSTALLFOLDER">
      <Component Id="MainExecutable" Guid="*">
        <File Id="AppExe" Source="bin\%s" KeyPath="yes" Checksum="yes"/>
      </Component>
    </ComponentGroup>
  </Fragment>
</Wix>
`, xmlEscape(spec.Name), xmlEscape(spec.Version), xmlEscape(spec.Name), xmlEscape(spec.AppID),
		xmlEscape(spec.Name), xmlEscape(spec.Name), xmlEscape(safeName), exeName)
	_ = arch // recorded in Spec / provenance; WiX Platform can be set at candle time
	if err := os.WriteFile(filepath.Join(outDir, "product.wxs"), []byte(wxs), 0o644); err != nil {
		return Artifact{}, err
	}
	art.Target = TargetWindowsMSI
	return art, nil
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
