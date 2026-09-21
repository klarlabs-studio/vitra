package shortcut_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/shortcut"
)

func TestShortcutPlugin_Contribute(t *testing.T) {
	p := shortcut.New()
	m := p.Manifest()
	if m.ID != shortcut.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "shortcut.register" {
		t.Fatalf("permissions: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 2 || string(c.Commands[0].Name()) != "shortcut.register" || string(c.Commands[1].Name()) != "shortcut.unregister" {
		t.Fatalf("commands: %v", c.Commands)
	}
	if string(c.Commands[1].Permission()) != "shortcut.register" {
		t.Fatalf("unregister permission: %s", c.Commands[1].Permission())
	}
	if len(c.Events) != 1 || string(c.Events[0]) != "shortcut.action" {
		t.Fatalf("events: %v", c.Events)
	}
}
