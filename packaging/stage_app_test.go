package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageDarwinApp_LayoutAndPlist(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	payload := []byte("mach-o-fake")
	if err := os.WriteFile(bin, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID:   "com.vitra.demo",
		Version: "1.2.3",
		Name:    "Demo App",
		Targets: []Target{TargetDarwinApp},
	}
	art, err := StageDarwinApp(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	if art.Target != TargetDarwinApp {
		t.Fatalf("target: %s", art.Target)
	}
	if art.SHA256 == "" {
		t.Fatal("missing sha256")
	}
	bundle := filepath.Join(out, "Demo-App.app")
	if art.Path != bundle {
		t.Fatalf("path: got %q want %q", art.Path, bundle)
	}
	exe := filepath.Join(bundle, "Contents", "MacOS", "Demo-App")
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch")
	}
	raw, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{
		"com.vitra.demo", "Demo App", "1.2.3", "Demo-App", "CFBundleExecutable", "APPL",
		"NSHumanReadableCopyright", DefaultLicense,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("plist missing %q\n%s", want, body)
		}
	}
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "Resources")); err != nil {
		t.Fatal(err)
	}
}

func TestStageDarwinApp_LicenseCopyright(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "stage")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.4.0", Name: "Demo",
		Targets: []Target{TargetDarwinApp}, License: "Apache-2.0",
	}
	art, err := StageDarwinApp(spec, bin, out)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(art.Path, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	want := "<key>NSHumanReadableCopyright</key>\n\t<string>Apache-2.0</string>"
	if !strings.Contains(body, want) {
		t.Fatalf("missing copyright\n%s", body)
	}

	outDefault := filepath.Join(tmp, "stage-default")
	spec.License = ""
	art, err = StageDarwinApp(spec, bin, outDefault)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(art.Path, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := "<key>NSHumanReadableCopyright</key>\n\t<string>" + DefaultLicense + "</string>"
	if !strings.Contains(string(raw), wantDefault) {
		t.Fatalf("missing default copyright\n%s", raw)
	}
}

func TestStageDarwinApp_OutIsBundlePath(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(tmp, "Custom.app")
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.1.0", Name: "Custom",
		Targets: []Target{TargetDarwinApp},
	}
	art, err := StageDarwinApp(spec, bin, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if art.Path != bundle {
		t.Fatalf("path: %q", art.Path)
	}
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "MacOS", "Custom")); err != nil {
		t.Fatal(err)
	}
}

func TestStageDarwinApp_RequiresDarwinTarget(t *testing.T) {
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.1.0", Name: "X",
		Targets: []Target{TargetLinuxDir},
	}
	_, err := StageDarwinApp(spec, "/bin/true", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "darwin-app") {
		t.Fatalf("got %v", err)
	}
}

func TestStageDarwinApp_EscapesPlist(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "app.bin")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		AppID: "com.vitra.demo", Version: "0.1.0", Name: `App & "Demo"`,
		Targets: []Target{TargetDarwinApp},
	}
	art, err := StageDarwinApp(spec, bin, tmp)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(art.Path, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "App &amp; &quot;Demo&quot;") {
		t.Fatalf("escape failed:\n%s", body)
	}
}
