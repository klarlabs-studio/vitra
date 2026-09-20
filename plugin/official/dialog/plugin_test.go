package dialog_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/dialog"
)

func TestDialogPlugin_Contribute(t *testing.T) {
	p := dialog.New()
	m := p.Manifest()
	if m.ID != dialog.PluginID {
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
