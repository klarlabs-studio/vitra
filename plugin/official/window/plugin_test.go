package window_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/window"
)

func TestWindowPlugin_Contribute(t *testing.T) {
	p := window.New()
	m := p.Manifest()
	if m.ID != window.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 2 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	if string(c.Commands[0].Name()) != "window.create" || string(c.Commands[1].Name()) != "window.close" {
		t.Fatalf("commands: %s %s", c.Commands[0].Name(), c.Commands[1].Name())
	}
}
