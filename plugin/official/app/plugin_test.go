package app_test

import (
	"testing"

	officialapp "go.klarlabs.de/vitra/plugin/official/app"
)

func TestAppPlugin_Contribute(t *testing.T) {
	p := officialapp.New()
	m := p.Manifest()
	if m.ID != officialapp.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "app.quit" {
		t.Fatalf("permissions: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 || string(c.Commands[0].Name()) != "app.quit" {
		t.Fatalf("commands: %v", c.Commands)
	}
}
