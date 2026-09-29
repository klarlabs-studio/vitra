package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageWindows_StagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.ico")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("ICO"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "1.0.0", Name: "Demo",
		Targets: []Target{TargetWindowsDir}, IconPath: icon,
	}
	if _, err := StageWindows(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "bin", "Demo.ico")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWiXDir_IncludesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.ico")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("ICO"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, IconPath: icon,
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{`SourceFile="bin\Demo.ico"`, "ARPPRODUCTICON", `Source="bin\Demo.ico"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}
}

func TestBuildNSISDir_IncludesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.ico")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("ICO"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, IconPath: icon,
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{`PRODUCT_ICON`, `File "bin\${PRODUCT_ICON}"`, `${PRODUCT_ICON}" 0`, `$DESKTOP\${PRODUCT_NAME}.lnk`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}
}

func TestStageWindows_LayoutAndDigest(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	payload := []byte("MZ-fake-windows-binary")
	if err := os.WriteFile(bin, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID:   "com.vitra.demo",
		Version: "1.2.3",
		Name:    "Demo App",
		Targets: []Target{TargetWindowsDir},
	}
	art, err := StageWindows(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != TargetWindowsDir {
		t.Fatalf("target: %s", art.Target)
	}
	if art.SHA256 == "" {
		t.Fatal("missing sha256")
	}
	exe := filepath.Join(out, "bin", "Demo-App.exe")
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestBuildWiXDir_WritesProductWXS(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID:   "com.vitra.demo",
		Version: "0.4.0",
		Name:    "Demo",
		Targets: []Target{TargetWindowsMSI},
	}
	art, err := BuildWiXDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != TargetWindowsMSI {
		t.Fatalf("target: %s", art.Target)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		"Demo", "0.4.0", `Source="bin\Demo.exe"`, "<Wix",
		"ProgramMenuFolder", "ApplicationProgramsFolder",
		"AppStartMenuShortcut", `Target="[INSTALLFOLDER]Demo.exe"`,
		"RemoveAppProgramsFolder",
		"DesktopFolder", "AppDesktopShortcut", "DesktopShortcut",
		"System.AppUserModel.ID", "com.vitra.demo",
		"UpgradeCode=\"{",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wxs missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, `UpgradeCode="com.vitra.demo"`) {
		t.Fatal("UpgradeCode should be a GUID, not raw AppID")
	}
	if _, err := os.Stat(filepath.Join(out, "bin", "Demo.exe")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWiXDir_UpgradeCodeStable(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI},
	}
	out1 := filepath.Join(tmp, "a")
	out2 := filepath.Join(tmp, "b")
	if _, err := BuildWiXDir(spec, bin, out1); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildWiXDir(spec, bin, out2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(out1, "product.wxs"))
	b, _ := os.ReadFile(filepath.Join(out2, "product.wxs"))
	if !strings.Contains(string(a), "UpgradeCode=") || string(a) != string(b) {
		t.Fatalf("UpgradeCode not stable across runs")
	}
}

func TestBuildWiXDir_ShortcutUsesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.ico")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("ICO"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, IconPath: icon,
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `Icon="AppIcon"`) {
		t.Fatalf("shortcut missing Icon=AppIcon\n%s", raw)
	}
}

func TestBuildNSISDir_WritesInstallerNSI(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID:   "com.vitra.demo",
		Version: "0.4.0",
		Name:    "Demo",
		Targets: []Target{TargetWindowsNSIS},
	}
	art, err := BuildNSISDir(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != TargetWindowsNSIS {
		t.Fatalf("target: %s", art.Target)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		"PRODUCT_NAME", "Demo", "0.4.0", "com.vitra.demo",
		`File "bin\${PRODUCT_EXE}"`,
		`WriteUninstaller "$INSTDIR\Uninstall.exe"`,
		`UNINST_KEY`,
		`WriteRegStr HKCU "${UNINST_KEY}" "DisplayName"`,
		`WriteRegStr HKCU "${UNINST_KEY}" "UninstallString"`,
		`DeleteRegKey HKCU "${UNINST_KEY}"`,
		`Delete "$INSTDIR\Uninstall.exe"`,
		`CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk"`,
		`Delete "$DESKTOP\${PRODUCT_NAME}.lnk"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("nsi missing %q\n%s", want, body)
		}
	}
}

func TestBuildNSISDir_CustomPublisher(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, Maintainer: "Acme Labs",
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `!define PRODUCT_PUBLISHER "Acme Labs"`) {
		t.Fatalf("publisher missing\n%s", raw)
	}
}

func TestBuildWiXDir_CustomManufacturer(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, Maintainer: "Acme Labs",
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `Manufacturer="Acme Labs"`) {
		t.Fatalf("manufacturer missing\n%s", raw)
	}
}

func TestBuildNSISDir_ARPDisplayIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	icon := filepath.Join(tmp, "app.ico")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("ICO"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, IconPath: icon,
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, `WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_ICON}"`) {
		t.Fatalf("missing DisplayIcon ARP entry\n%s", body)
	}
}

func TestBuildNSISDir_HomepageURLInfoAbout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, Homepage: "https://example.com/demo",
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	want := `WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "https://example.com/demo"`
	if !strings.Contains(body, want) {
		t.Fatalf("missing URLInfoAbout\n%s", body)
	}
	wantHelp := `WriteRegStr HKCU "${UNINST_KEY}" "HelpLink" "https://example.com/demo"`
	if !strings.Contains(body, wantHelp) {
		t.Fatalf("missing HelpLink\n%s", body)
	}
	wantUpdate := `WriteRegStr HKCU "${UNINST_KEY}" "URLUpdateInfo" "https://example.com/demo"`
	if !strings.Contains(body, wantUpdate) {
		t.Fatalf("missing URLUpdateInfo\n%s", body)
	}

	outEmpty := filepath.Join(tmp, "nsis-empty")
	spec.Homepage = ""
	if _, err := BuildNSISDir(spec, bin, outEmpty); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outEmpty, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "URLInfoAbout") || strings.Contains(string(raw), "HelpLink") || strings.Contains(string(raw), "URLUpdateInfo") {
		t.Fatalf("unexpected URLInfoAbout/HelpLink/URLUpdateInfo when Homepage empty\n%s", raw)
	}
}

func TestBuildWiXDir_HomepageARPURL(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, Homepage: "https://example.com/demo",
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	want := `<Property Id="ARPURLINFOABOUT" Value="https://example.com/demo"/>`
	if !strings.Contains(body, want) {
		t.Fatalf("missing ARPURLINFOABOUT\n%s", body)
	}
	wantHelp := `<Property Id="ARPHELPLINK" Value="https://example.com/demo"/>`
	if !strings.Contains(body, wantHelp) {
		t.Fatalf("missing ARPHELPLINK\n%s", body)
	}
	wantUpdate := `<Property Id="ARPURLUPDATEINFO" Value="https://example.com/demo"/>`
	if !strings.Contains(body, wantUpdate) {
		t.Fatalf("missing ARPURLUPDATEINFO\n%s", body)
	}

	outEmpty := filepath.Join(tmp, "wix-empty")
	spec.Homepage = ""
	if _, err := BuildWiXDir(spec, bin, outEmpty); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outEmpty, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ARPURLINFOABOUT") || strings.Contains(string(raw), "ARPHELPLINK") || strings.Contains(string(raw), "ARPURLUPDATEINFO") {
		t.Fatalf("unexpected ARPURLINFOABOUT/ARPHELPLINK/ARPURLUPDATEINFO when Homepage empty\n%s", raw)
	}
}

func TestBuildNSISDir_ARPComments(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, Description: "Demo desktop app",
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	want := `WriteRegStr HKCU "${UNINST_KEY}" "Comments" "Demo desktop app"`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("missing Comments\n%s", raw)
	}

	outDefault := filepath.Join(tmp, "nsis-default")
	spec.Description = ""
	if _, err := BuildNSISDir(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outDefault, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := `WriteRegStr HKCU "${UNINST_KEY}" "Comments" "` + DefaultDescription + `"`
	if !strings.Contains(string(raw), wantDefault) {
		t.Fatalf("missing default Comments\n%s", raw)
	}
}

func TestBuildWiXDir_ARPComments(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, Description: "Demo desktop app",
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		`<Property Id="ARPCOMMENTS" Value="Demo desktop app"/>`,
		`Description="Demo desktop app" Comments="Demo desktop app"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}

	outDefault := filepath.Join(tmp, "wix-default")
	spec.Description = ""
	if _, err := BuildWiXDir(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outDefault, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body = string(raw)
	wantDefault := `<Property Id="ARPCOMMENTS" Value="` + DefaultDescription + `"/>`
	if !strings.Contains(body, wantDefault) {
		t.Fatalf("missing default ARPCOMMENTS\n%s", body)
	}
	if strings.Contains(body, "Vitra-packaged Windows installer") {
		t.Fatalf("stale hardcoded Package Comments\n%s", body)
	}
}

func TestBuildNSISDir_ARPLegalCopyright(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS}, License: "Apache-2.0",
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	want := `WriteRegStr HKCU "${UNINST_KEY}" "LegalCopyright" "Apache-2.0"`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("missing LegalCopyright\n%s", raw)
	}

	outDefault := filepath.Join(tmp, "nsis-default")
	spec.License = ""
	if _, err := BuildNSISDir(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outDefault, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := `WriteRegStr HKCU "${UNINST_KEY}" "LegalCopyright" "` + DefaultLicense + `"`
	if !strings.Contains(string(raw), wantDefault) {
		t.Fatalf("missing default LegalCopyright\n%s", raw)
	}
}

func TestBuildWiXDir_ARPCopyright(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI}, License: "Apache-2.0",
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<Property Id="ARPCOPYRIGHT" Value="Apache-2.0"/>`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("missing ARPCOPYRIGHT\n%s", raw)
	}

	outDefault := filepath.Join(tmp, "wix-default")
	spec.License = ""
	if _, err := BuildWiXDir(spec, bin, outDefault); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(outDefault, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := `<Property Id="ARPCOPYRIGHT" Value="` + DefaultLicense + `"/>`
	if !strings.Contains(string(raw), wantDefault) {
		t.Fatalf("missing default ARPCOPYRIGHT\n%s", raw)
	}
}

func TestBuildNSISDir_ARPNoModifyNoRepair(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS},
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		`WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1`,
		`WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}
}

func TestBuildWiXDir_ARPNoModifyNoRepair(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI},
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		`<Property Id="ARPNOMODIFY" Value="1"/>`,
		`<Property Id="ARPNOREPAIR" Value="1"/>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q\n%s", want, body)
		}
	}
}

func TestBuildNSISDir_ARPEstimatedSize(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	// 2048 bytes → 2 KB after (size+1023)/1024 rounding.
	payload := make([]byte, 2048)
	copy(payload, []byte("MZ"))
	if err := os.WriteFile(bin, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "nsis")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsNSIS},
	}
	if _, err := BuildNSISDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "installer.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	want := `WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" 2`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("missing EstimatedSize\n%s", raw)
	}
}

func TestBuildWiXDir_ARPEstimatedSize(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	payload := make([]byte, 2048)
	copy(payload, []byte("MZ"))
	if err := os.WriteFile(bin, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "wix")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetWindowsMSI},
	}
	if _, err := BuildWiXDir(spec, bin, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "product.wxs"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<Property Id="ARPSIZE" Value="2"/>`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("missing ARPSIZE\n%s", raw)
	}
}

func TestWindowsARPEstimatedSizeKB(t *testing.T) {
	if got := windowsARPEstimatedSizeKB("/no/such/file", ""); got != 1 {
		t.Fatalf("missing binary floor: %d", got)
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, make([]byte, 1025), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := windowsARPEstimatedSizeKB(bin, ""); got != 2 {
		t.Fatalf("1025 bytes → want 2 KB, got %d", got)
	}
}
