package packaging_test

import (
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
	if !strings.Contains(joined, "plan only") {
		t.Fatalf("expected plan-only note: %s", joined)
	}
	if strings.Contains(joined, "demo-app") == false && !strings.Contains(joined, "register") {
		t.Fatalf("expected register step: %s", joined)
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
