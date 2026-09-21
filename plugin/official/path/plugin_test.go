package path_test

import (
	"testing"

	officialpath "go.klarlabs.de/vitra/plugin/official/path"
)

func TestPathPlugin_Contribute(t *testing.T) {
	p := officialpath.New()
	m := p.Manifest()
	if m.ID != officialpath.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "path.open" {
		t.Fatalf("perms: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 || c.Commands[0].Name() != "path.open" {
		t.Fatalf("commands: %+v", c.Commands)
	}
}
