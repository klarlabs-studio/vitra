package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestPlanSign_DarwinCodesign(t *testing.T) {
	t.Setenv("APPLE_IDENTITY", "Developer ID Application: Example")
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetDarwinApp},
		Sign:               true,
		SigningIdentityRef: "env:APPLE_IDENTITY",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/Demo.app")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "codesign" || !plan.IdentityOK {
		t.Fatalf("%+v", plan)
	}
	joined := strings.Join(plan.Args, " ")
	if !strings.Contains(joined, "--sign ${APPLE_IDENTITY}") || !strings.Contains(joined, "/tmp/Demo.app") {
		t.Fatalf("args=%v", plan.Args)
	}
	if strings.Contains(plan.String(), "Developer ID Application") {
		t.Fatalf("plan leaked env value: %s", plan.String())
	}
	if !strings.Contains(plan.String(), "plan only until ExecuteSign") {
		t.Fatalf("plan=%s", plan.String())
	}
	if len(plan.FollowUps) != 2 {
		t.Fatalf("expected notarize+staple follow-ups, got %+v", plan.FollowUps)
	}
	if plan.FollowUps[0].Tool != "notarytool" || plan.FollowUps[1].Tool != "stapler" {
		t.Fatalf("follow-ups=%+v", plan.FollowUps)
	}
	joinedFU := strings.Join(plan.FollowUps[0].Args, " ")
	if !strings.Contains(joinedFU, "submit /tmp/Demo.app") || !strings.Contains(joinedFU, "--keychain-profile") {
		t.Fatalf("notarize args=%v", plan.FollowUps[0].Args)
	}
	if !strings.Contains(plan.String(), "follow-up 1:") || !strings.Contains(plan.String(), "notarytool") {
		t.Fatalf("string missing follow-ups: %s", plan.String())
	}
}

func TestPlanSign_DarwinNotarizeProfileFromKeychain(t *testing.T) {
	_ = os.Unsetenv("NOTARYTOOL_PROFILE")
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetDarwinDMG},
		Sign:               true,
		SigningIdentityRef: "keychain:AC_PASSWORD",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/Demo.dmg")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.FollowUps) == 0 {
		t.Fatal("expected follow-ups")
	}
	joined := strings.Join(plan.FollowUps[0].Args, " ")
	if !strings.Contains(joined, "--keychain-profile AC_PASSWORD") {
		t.Fatalf("expected keychain profile name, got %v", plan.FollowUps[0].Args)
	}
}

func TestPlanSign_WindowsSigntoolFile(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.p12")
	if err := os.WriteFile(cert, []byte("pkcs12"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetWindowsMSI},
		Sign:               true,
		SigningIdentityRef: "file:" + cert,
	}
	plan, err := packaging.PlanSign(spec, filepath.Join(dir, "Demo.msi"))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "signtool" || !plan.IdentityOK {
		t.Fatalf("%+v", plan)
	}
	joined := strings.Join(plan.Args, " ")
	if !strings.Contains(joined, "/f "+cert) || !strings.Contains(joined, "Demo.msi") {
		t.Fatalf("args=%v", plan.Args)
	}
}

func TestPlanSign_LinuxUnsupported(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxDir},
		Sign:               true,
		SigningIdentityRef: "secret:ci/gpg",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Supported || plan.Tool != "" {
		t.Fatalf("%+v", plan)
	}
	if !strings.Contains(plan.Note, "not orchestrated") {
		t.Fatalf("note=%q", plan.Note)
	}
}

func TestPlanSign_LinuxDeb(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxDeb},
		Sign:               true,
		SigningIdentityRef: "env:GPG_KEY_ID",
	}
	t.Setenv("GPG_KEY_ID", "ABCDEF01")
	plan, err := packaging.PlanSign(spec, "/tmp/demo.deb")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "dpkg-sig" {
		t.Fatalf("%+v", plan)
	}
	joined := strings.Join(plan.Args, " ")
	if !strings.Contains(joined, "-k ${GPG_KEY_ID}") || !strings.Contains(joined, "/tmp/demo.deb") {
		t.Fatalf("args=%v", plan.Args)
	}
	if strings.Contains(plan.String(), "ABCDEF01") {
		t.Fatalf("leaked key id: %s", plan.String())
	}
}

func TestPlanSign_LinuxRPM(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxRPM},
		Sign:               true,
		SigningIdentityRef: "keychain:rpm-packager",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo.rpm")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "rpmsign" {
		t.Fatalf("%+v", plan)
	}
	if !strings.Contains(plan.Note, "rpm-packager") || !strings.Contains(strings.Join(plan.Args, " "), "--addsign") {
		t.Fatalf("%+v", plan)
	}
}

func TestPlanSign_LinuxSnapAndGPG(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxSnap},
		Sign:               true,
		SigningIdentityRef: "secret:snap-store",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo.snap")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "snapcraft" {
		t.Fatalf("%+v", plan)
	}
	spec.Targets = []packaging.Target{packaging.TargetLinuxFlatpak}
	plan, err = packaging.PlanSign(spec, "/tmp/demo.flatpak")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Tool != "gpg" {
		t.Fatalf("%+v", plan)
	}
}

func TestPlanSign_RequiresSign(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetDarwinDMG},
	}
	if _, err := packaging.PlanSign(spec, "/tmp/x.dmg"); err == nil {
		t.Fatal("expected error when Sign=false")
	}
}

func TestPlanSign_UnsetEnv(t *testing.T) {
	t.Setenv("MISSING_SIGN_ID", "")
	_ = os.Unsetenv("MISSING_SIGN_ID")
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetDarwinDMG},
		Sign:               true,
		SigningIdentityRef: "env:MISSING_SIGN_ID",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/x.dmg")
	if err != nil {
		t.Fatal(err)
	}
	if plan.IdentityOK {
		t.Fatalf("expected IdentityOK=false: %+v", plan)
	}
	if !strings.Contains(plan.String(), "warning") {
		t.Fatalf("plan=%s", plan.String())
	}
}

func TestResolveCodesign_Missing(t *testing.T) {
	t.Setenv("VITRA_CODESIGN", filepath.Join(t.TempDir(), "missing"))
	_, err := packaging.ResolveCodesign()
	if err == nil || !strings.Contains(err.Error(), "VITRA_CODESIGN") {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteSign_RunsPrimaryWithEnvExpansion(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv.log")
	tool := filepath.Join(dir, "fake-dpkg-sig")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > " + logPath + "\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_DPKGSIG", tool)
	t.Setenv("GPG_KEY_ID", "DEADBEEF")

	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxDeb},
		Sign:               true,
		SigningIdentityRef: "env:GPG_KEY_ID",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo.deb")
	if err != nil {
		t.Fatal(err)
	}
	if err := packaging.ExecuteSign(plan, packaging.ExecuteSignOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(got))
	if !strings.Contains(line, "DEADBEEF") || !strings.Contains(line, "/tmp/demo.deb") {
		t.Fatalf("argv log=%q", line)
	}
	if strings.Contains(line, "${") {
		t.Fatalf("placeholder not expanded: %q", line)
	}
}

func TestExecuteSign_FollowUpsAndSecretReject(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	makeTool := func(name string) string {
		p := filepath.Join(dir, name)
		script := "#!/bin/sh\necho \"" + name + " $*\" >> " + logPath + "\n"
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	t.Setenv("VITRA_CODESIGN", makeTool("fake-codesign"))
	t.Setenv("VITRA_NOTARYTOOL", makeTool("fake-notarytool"))
	t.Setenv("VITRA_STAPLER", makeTool("fake-stapler"))
	t.Setenv("APPLE_IDENTITY", "Developer ID Application: Example")
	t.Setenv("NOTARYTOOL_PROFILE", "AC_PROFILE")

	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetDarwinDMG},
		Sign:               true,
		SigningIdentityRef: "env:APPLE_IDENTITY",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/Demo.dmg")
	if err != nil {
		t.Fatal(err)
	}
	if err := packaging.ExecuteSign(plan, packaging.ExecuteSignOptions{FollowUps: true}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"fake-codesign", "fake-notarytool", "fake-stapler", "Developer ID Application: Example", "AC_PROFILE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q:\n%s", want, text)
		}
	}

	secretPlan := plan
	secretPlan.IdentityRef = "secret:ci/apple"
	secretPlan.Args = []string{"--sign", "<secret:ci/apple>", "/tmp/Demo.dmg"}
	_ = os.Unsetenv("VITRA_SECRET_CI_APPLE")
	if err := packaging.ExecuteSign(secretPlan, packaging.ExecuteSignOptions{}); err == nil || !strings.Contains(err.Error(), "VITRA_SECRET_CI_APPLE") {
		t.Fatalf("expected unset secret reject, got %v", err)
	}
	t.Setenv("VITRA_SECRET_CI_APPLE", "Developer ID Application: FromSecret")
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := packaging.ExecuteSign(secretPlan, packaging.ExecuteSignOptions{}); err != nil {
		t.Fatal(err)
	}
	got2, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got2), "Developer ID Application: FromSecret") {
		t.Fatalf("secret not expanded: %s", got2)
	}
	if strings.Contains(string(got2), "<secret:") {
		t.Fatalf("placeholder leaked into argv: %s", got2)
	}
}

func TestSecretEnvKey(t *testing.T) {
	// Exercised via PlanSign note + ExecuteSign; keep display opaque in plans.
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets:            []packaging.Target{packaging.TargetLinuxDeb},
		Sign:               true,
		SigningIdentityRef: "secret:ci/gpg-key",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo.deb")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	if !strings.Contains(joined, "<secret:ci/gpg-key>") {
		t.Fatalf("expected opaque placeholder in argv: %v", plan.Args)
	}
	if !strings.Contains(plan.String(), "VITRA_SECRET_CI_GPG_KEY") {
		t.Fatalf("IdentityNote should mention env key: %s", plan.String())
	}
}
