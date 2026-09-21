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
	if len(c.Commands) != 15 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	want := []string{
		"window.create", "window.close", "window.chrome", "window.getChrome",
		"window.focus", "window.blur", "window.hide", "window.show",
		"window.minimize", "window.maximize", "window.fullscreen",
		"window.setAlwaysOnTop", "window.restore", "window.setTitle", "window.setSize",
	}
	for i, name := range want {
		if string(c.Commands[i].Name()) != name {
			t.Fatalf("command[%d]=%s want %s", i, c.Commands[i].Name(), name)
		}
	}
	for i, name := range []string{"getChrome", "focus", "blur", "hide", "show", "minimize", "maximize", "fullscreen", "setAlwaysOnTop", "restore", "setTitle", "setSize"} {
		idx := i + 3
		if string(c.Commands[idx].Permission()) != "window.chrome" {
			t.Fatalf("%s permission: %s", name, c.Commands[idx].Permission())
		}
	}
}
