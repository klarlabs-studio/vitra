package clipboard_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/clipboard"
)

func TestClipboardPlugin_Contribute(t *testing.T) {
	p := clipboard.New()
	m := p.Manifest()
	if m.ID != clipboard.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 2 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
}
