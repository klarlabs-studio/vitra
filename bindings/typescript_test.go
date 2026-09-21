package bindings_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/vitra/bindings"
	"go.klarlabs.de/vitra/domain"
)

func TestGenerateTypeScript(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("project.open", "Open a project", "fs.read")
	out := bindings.GenerateTypeScript("app", "0.3.0", []*domain.CommandDefinition{cmd}, nil)
	for _, want := range []string{
		"kernel: 0.3.0",
		"projectOpen",
		"fs.read",
		`invoke("project.open"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "VitraEvents") {
		t.Fatal("expected no events section without events")
	}
}

func TestGenerateTypeScript_Events(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("fs.read", "Read a file", "fs.read")
	out := bindings.GenerateTypeScript("vitra", "0.4.0", []*domain.CommandDefinition{cmd}, []domain.EventName{
		"fs.changed", "demo.tick", "fs.changed",
	})
	for _, want := range []string{
		"VitraEvents",
		"createEvents",
		"onFsChanged",
		"onDemoTick",
		`on("fs.changed"`,
		`on("demo.tick"`,
		"VitraSubscriber",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Dedupe: only one onFsChanged line in the return object.
	if strings.Count(out, `on("fs.changed"`) != 1 {
		t.Fatalf("expected deduped fs.changed:\n%s", out)
	}
}
