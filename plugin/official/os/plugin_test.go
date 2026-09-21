package os_test

import (
	"testing"

	officialos "go.klarlabs.de/vitra/plugin/official/os"
)

func TestOsPlugin_Contribute(t *testing.T) {
	p := officialos.New()
	m := p.Manifest()
	if m.ID != officialos.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "os.info" {
		t.Fatalf("perms: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	if c.Commands[0].Name() != "os.info" {
		t.Fatalf("command name: %s", c.Commands[0].Name())
	}
}
