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
	if len(c.Commands) != 4 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	want := []string{"dialog.open", "dialog.save", "dialog.openDirectory", "dialog.message"}
	for i, name := range want {
		if string(c.Commands[i].Name()) != name {
			t.Fatalf("command[%d]=%s want %s", i, c.Commands[i].Name(), name)
		}
	}
}
