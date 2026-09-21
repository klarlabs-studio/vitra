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
	if len(m.Permissions) != 3 {
		t.Fatalf("permissions: %d", len(m.Permissions))
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 3 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	want := []string{"window.create", "window.close", "window.chrome"}
	for i, name := range want {
		if string(m.Permissions[i]) != name {
			t.Fatalf("permission[%d]=%s want %s", i, m.Permissions[i], name)
		}
		if string(c.Commands[i].Name()) != name {
			t.Fatalf("command[%d]=%s want %s", i, c.Commands[i].Name(), name)
		}
	}
}
