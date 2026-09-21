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
	if len(c.Commands) != 10 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	want := []string{"window.create", "window.close", "window.chrome", "window.getChrome", "window.focus", "window.blur", "window.hide", "window.show", "window.minimize", "window.maximize"}
	for i, name := range want {
		if string(c.Commands[i].Name()) != name {
			t.Fatalf("command[%d]=%s want %s", i, c.Commands[i].Name(), name)
		}
	}
	if string(c.Commands[3].Permission()) != "window.chrome" {
		t.Fatalf("getChrome permission: %s", c.Commands[3].Permission())
	}
	if string(c.Commands[4].Permission()) != "window.chrome" {
		t.Fatalf("focus permission: %s", c.Commands[4].Permission())
	}
	if string(c.Commands[5].Permission()) != "window.chrome" {
		t.Fatalf("blur permission: %s", c.Commands[5].Permission())
	}
	if string(c.Commands[6].Permission()) != "window.chrome" {
		t.Fatalf("hide permission: %s", c.Commands[6].Permission())
	}
	if string(c.Commands[7].Permission()) != "window.chrome" {
		t.Fatalf("show permission: %s", c.Commands[7].Permission())
	}
	if string(c.Commands[8].Permission()) != "window.chrome" {
		t.Fatalf("minimize permission: %s", c.Commands[8].Permission())
	}
	if string(c.Commands[9].Permission()) != "window.chrome" {
		t.Fatalf("maximize permission: %s", c.Commands[9].Permission())
	}
}
