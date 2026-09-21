package tray_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/tray"
)

func TestTrayPlugin_Contribute(t *testing.T) {
	p := tray.New()
	m := p.Manifest()
	if m.ID != tray.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "tray.set" {
		t.Fatalf("permissions: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 || string(c.Commands[0].Name()) != "tray.set" {
		t.Fatalf("commands: %v", c.Commands)
	}
	if len(c.Events) != 1 || string(c.Events[0]) != "tray.action" {
		t.Fatalf("events: %v", c.Events)
	}
}
