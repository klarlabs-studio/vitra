package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestPlanPublish_Snap(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo App",
		Targets: []packaging.Target{packaging.TargetLinuxSnap},
	}
	plan, err := packaging.PlanPublish(spec, "/tmp/demo.snap")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Store != "Snap Store" || len(plan.Steps) != 3 {
		t.Fatalf("%+v", plan)
	}
	joined := plan.String()
	if !strings.Contains(joined, "snapcraft upload /tmp/demo.snap") {
		t.Fatalf("plan=%s", joined)
	}
	if !strings.Contains(joined, "ExecutePublish") || !strings.Contains(joined, "--publish-execute") {
		t.Fatalf("expected ExecutePublish note: %s", joined)
	}
	if strings.Contains(joined, "demo-app") == false && !strings.Contains(joined, "register") {
		t.Fatalf("expected register step: %s", joined)
	}
	upload := plan.Steps[2]
	if !upload.Executable || upload.Args[0] != "upload" {
		t.Fatalf("upload step: %+v", upload)
	}
	if plan.Steps[0].Executable || plan.Steps[1].Executable {
		t.Fatal("login/register must not be executable")
	}
}

func TestPlanPublish_Flatpak(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.Demo", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxFlatpak},
	}
	plan, err := packaging.PlanPublish(spec, "/tmp/demo.flatpak")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || plan.Store != "Flathub" || len(plan.Steps) < 3 {
		t.Fatalf("%+v", plan)
	}
	joined := plan.String()
	for _, want := range []string{"flathub", "flatpak-builder", "build-bundle", "com.example.Demo"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	execCount := 0
	for _, step := range plan.Steps {
		if step.Executable {
			execCount++
			if step.Tool != "flatpak-builder" {
				t.Fatalf("unexpected executable tool %q", step.Tool)
			}
		}
		if step.Tool == "gh" && step.Executable {
			t.Fatal("gh pr must not be executable")
		}
	}
	if execCount != 1 {
		t.Fatalf("executable steps=%d", execCount)
	}
}

func TestPlanPublish_Unsupported(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
	}
	plan, err := packaging.PlanPublish(spec, "/tmp/demo.deb")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Supported || plan.Store != "" {
		t.Fatalf("%+v", plan)
	}
	if !strings.Contains(plan.Note, "no first-party store") {
		t.Fatalf("note=%q", plan.Note)
	}
}

func TestPlanPublish_RequiresArtifact(t *testing.T) {
	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxSnap},
	}
	if _, err := packaging.PlanPublish(spec, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestExecutePublish_SnapUploadOnly(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv.log")
	tool := filepath.Join(dir, "fake-snapcraft")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_SNAPCRAFT", tool)

	spec := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo App",
		Targets: []packaging.Target{packaging.TargetLinuxSnap},
	}
	plan, err := packaging.PlanPublish(spec, "/tmp/demo.snap")
	if err != nil {
		t.Fatal(err)
	}
	if err := packaging.ExecutePublish(plan, packaging.ExecutePublishOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(got))
	if !strings.Contains(line, "upload /tmp/demo.snap") {
		t.Fatalf("argv log=%q", line)
	}
	if strings.Contains(line, "login") || strings.Contains(line, "register") {
		t.Fatalf("interactive steps must not run: %q", line)
	}
}

func TestExecutePublish_RejectsUnsupportedAndSkipsGH(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	tool := filepath.Join(dir, "fake-flatpak-builder")
	script := "#!/bin/sh\necho flatpak-builder \"$*\" >> " + logPath + "\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_FLATPAK_BUILDER", tool)

	spec := packaging.Spec{
		AppID: "com.example.Demo", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxFlatpak},
	}
	plan, err := packaging.PlanPublish(spec, "/tmp/demo.flatpak")
	if err != nil {
		t.Fatal(err)
	}
	if err := packaging.ExecutePublish(plan, packaging.ExecutePublishOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "flatpak-builder") || strings.Contains(body, "gh ") || strings.Contains(body, "git ") {
		t.Fatalf("log=%q", body)
	}

	deb := packaging.Spec{
		AppID: "com.example.app", Version: "1.0.0", Name: "Demo",
		Targets: []packaging.Target{packaging.TargetLinuxDeb},
	}
	debPlan, err := packaging.PlanPublish(deb, "/tmp/demo.deb")
	if err != nil {
		t.Fatal(err)
	}
	if err := packaging.ExecutePublish(debPlan, packaging.ExecutePublishOptions{}); err == nil {
		t.Fatal("expected unsupported reject")
	}
}
