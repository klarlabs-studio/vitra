package deeplink_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/deeplink"
)

func TestDeeplinkPlugin_Contribute(t *testing.T) {
	p := deeplink.New()
	m := p.Manifest()
	if m.ID != deeplink.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "deeplink.handle" {
		t.Fatalf("permissions: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 0 {
		t.Fatalf("commands: %v", c.Commands)
	}
	if len(c.Events) != 1 || string(c.Events[0]) != "deeplink.open" {
		t.Fatalf("events: %v", c.Events)
	}
}
