package provenance_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/provenance"
)

func TestDocument_JSONIncludesPrivilegedSurface(t *testing.T) {
	doc := provenance.NewDocument("com.example.app", "1.0.0", "go1.26.7")
	doc.Plugins = []provenance.PluginInfo{{
		ID: "vitra.fs", Version: "1.0.0", Perms: []string{"fs.read", "fs.write"},
	}}
	doc.Capabilities = []string{"fs.read", "dialog.open"}
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
	for _, want := range []string{"vitra.fs", "fs.read", "artifact_sha256"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}
