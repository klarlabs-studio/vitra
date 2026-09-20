package plugin_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

type stubPlugin struct {
	manifest plugin.Manifest
	commands []*domain.CommandDefinition
}

func (s stubPlugin) Manifest() plugin.Manifest { return s.manifest }
func (s stubPlugin) Contribute() (plugin.Contribution, error) {
	return plugin.Contribution{Commands: s.commands}, nil
}

func TestRegistry_RejectsPermissionCollision(t *testing.T) {
	// Security invariant 6: a plugin cannot silently expand another plugin's scope.
	reg := plugin.NewRegistry(plugin.SemVer{Major: 0, Minor: 3, Patch: 0})
	cmd, _ := domain.NewCommandDefinition("fs.read", "Read file", "fs.read")
	a := stubPlugin{
		manifest: plugin.Manifest{
			ID: "fs", Name: "Filesystem", Version: plugin.SemVer{Major: 1},
			Permissions: []domain.PermissionName{"fs.read"},
			MinKernel:   plugin.SemVer{Major: 0, Minor: 1},
		},
		commands: []*domain.CommandDefinition{cmd},
	}
	if err := reg.Register(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	b := stubPlugin{
		manifest: plugin.Manifest{
			ID: "fs-evil", Name: "Evil FS", Version: plugin.SemVer{Major: 1},
			Permissions: []domain.PermissionName{"fs.read"},
			MinKernel:   plugin.SemVer{Major: 0, Minor: 1},
		},
		commands: []*domain.CommandDefinition{cmd},
	}
	err := reg.Register(context.Background(), b)
	var conflict *domain.ErrConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	owner, ok := reg.OwnerOf("fs.read")
	if !ok || owner != "fs" {
		t.Fatalf("owner=%s ok=%v", owner, ok)
	}
}

func TestRegistry_RejectsUndeclaredCommandPermission(t *testing.T) {
	reg := plugin.NewRegistry(plugin.SemVer{Major: 0, Minor: 3})
	cmd, _ := domain.NewCommandDefinition("shell.exec", "Exec", "shell.exec")
	p := stubPlugin{
		manifest: plugin.Manifest{
			ID: "shell", Name: "Shell", Version: plugin.SemVer{Major: 1},
			Permissions: []domain.PermissionName{"shell.list"}, // does not declare shell.exec
			MinKernel:   plugin.SemVer{Major: 0, Minor: 1},
		},
		commands: []*domain.CommandDefinition{cmd},
	}
	err := reg.Register(context.Background(), p)
	if err == nil {
		t.Fatal("expected undeclared permission error")
	}
}

func TestSemVer_Compatible(t *testing.T) {
	v := plugin.SemVer{Major: 1, Minor: 2, Patch: 3}
	if !v.CompatibleWith(plugin.SemVer{Major: 1, Minor: 2, Patch: 0}) {
		t.Fatal("expected compatible")
	}
	if v.CompatibleWith(plugin.SemVer{Major: 2}) {
		t.Fatal("major mismatch")
	}
}
