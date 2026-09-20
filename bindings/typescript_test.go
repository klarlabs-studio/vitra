package bindings_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/vitra/bindings"
	"go.klarlabs.de/vitra/domain"
)

func TestGenerateTypeScript(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("project.open", "Open a project", "fs.read")
	out := bindings.GenerateTypeScript("app", "0.3.0", []*domain.CommandDefinition{cmd})
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
}
