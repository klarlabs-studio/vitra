package fs_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/fs"
)

func TestFSPlugin_Contribute(t *testing.T) {
	p := fs.New()
	m := p.Manifest()
	if m.ID != fs.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) < 1 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
}
