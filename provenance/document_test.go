package provenance_test

import (
	"encoding/json"
	"runtime/debug"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/provenance"
)

func TestDocument_JSONIncludesPrivilegedSurface(t *testing.T) {
	doc := provenance.NewDocument("com.example.app", "1.0.0", "go1.26.7")
	doc = doc.WithPluginInventory([]provenance.PluginInfo{{
		ID: "vitra.fs", Version: "1.0.0", Perms: []string{"fs.read", "fs.write"},
	}, {
		ID: "vitra.dialog", Version: "1.0.0", Perms: []string{"dialog.open"},
	}})
	doc = doc.WithArtifactDigest([]byte("artifact"))
	raw, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"vitra.fs", "fs.read", "dialog.open", "artifact_sha256"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	caps, _ := m["capabilities"].([]any)
	if len(caps) != 3 {
		t.Fatalf("capabilities=%v", caps)
	}
}

func TestWithModulesFromBuildInfo(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Path: "go.klarlabs.de/vitra", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: "example.com/dep", Version: "v1.2.3"},
		},
	}
	doc := provenance.NewDocument("com.example.app", "1.0.0", "go1.26.7").
		WithModulesFromBuildInfo(bi)
	raw, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"go.klarlabs.de/vitra", "example.com/dep", "v1.2.3"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	nilDoc := provenance.NewDocument("a", "1", "go").WithModulesFromBuildInfo(nil)
	if len(nilDoc.Modules) != 0 {
		t.Fatalf("nil bi should leave modules empty: %+v", nilDoc.Modules)
	}
}
