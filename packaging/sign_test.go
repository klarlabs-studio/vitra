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
	if !strings.Contains(plan.String(), "dry-run only") {
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
		Targets:            []packaging.Target{packaging.TargetLinuxDeb},
		Sign:               true,
		SigningIdentityRef: "secret:ci/gpg",
	}
	plan, err := packaging.PlanSign(spec, "/tmp/demo.deb")
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
