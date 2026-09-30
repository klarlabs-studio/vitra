package official_test

import (
	"context"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official"
)

func TestAll_ContributesDistinctCommandsAndPermissions(t *testing.T) {
	ids := map[domain.PluginID]bool{}
	for _, p := range official.All() {
		m := p.Manifest()
		if err := m.Validate(); err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
		if ids[m.ID] || !strings.HasPrefix(string(m.ID), "vitra.") {
			t.Fatalf("duplicate or unprefixed id %q", m.ID)
		}
		ids[m.ID] = true
		c, err := p.Contribute()
		if err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
		if len(c.Commands) == 0 && len(c.Events) == 0 {
			t.Fatalf("%s contributes neither commands nor events", m.ID)
		}
		declared := map[domain.PermissionName]bool{}
		for _, perm := range m.Permissions {
			declared[perm] = true
		}
		for _, cmd := range c.Commands {
			if !declared[cmd.Permission()] {
				t.Fatalf("%s: command %s needs undeclared permission %s", m.ID, cmd.Name(), cmd.Permission())
			}
		}
	}
	if len(ids) != 14 {
		t.Fatalf("All() has %d plugins, want 14", len(ids))
	}
}

// Security invariant 6: official plugins must not claim each other's
// permissions, so they register together without collisions.
func TestAll_RegisterCleanly(t *testing.T) {
	reg := plugin.NewRegistry(plugin.SemVer{Major: 0, Minor: 3, Patch: 0})
	for _, p := range official.All() {
		if err := reg.Register(context.Background(), p); err != nil {
			t.Fatalf("%s: %v", p.Manifest().ID, err)
		}
	}
	for id, p := range map[domain.PluginID]plugin.Plugin{
		official.FSID: official.FS(), official.WindowID: official.Window(), official.DeepLinkID: official.DeepLink(),
	} {
		if p.Manifest().ID != id {
			t.Fatalf("constructor/id mismatch for %s", id)
		}
	}
}
