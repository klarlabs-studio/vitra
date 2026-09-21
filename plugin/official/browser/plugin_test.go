package browser_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/browser"
)

func TestBrowserPlugin_Contribute(t *testing.T) {
	p := browser.New()
	m := p.Manifest()
	if m.ID != browser.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "browser.open" {
		t.Fatalf("perms: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	if c.Commands[0].Name() != "browser.open" {
		t.Fatalf("command name: %s", c.Commands[0].Name())
	}
}
