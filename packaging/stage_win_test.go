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
