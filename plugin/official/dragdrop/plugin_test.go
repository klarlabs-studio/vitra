package dragdrop_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/dragdrop"
)

func TestDragDropPlugin_Contribute(t *testing.T) {
	p := dragdrop.New()
	m := p.Manifest()
	if m.ID != dragdrop.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "dragdrop.receive" {
		t.Fatalf("permissions: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 || string(c.Commands[0].Name()) != "dragdrop.receive" {
		t.Fatalf("commands: %v", c.Commands)
	}
	if len(c.Events) != 1 || string(c.Events[0]) != "dragdrop.drop" {
		t.Fatalf("events: %v", c.Events)
	}
}
