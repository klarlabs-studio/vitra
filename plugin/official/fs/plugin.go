// Package fs is the official Phase 3 filesystem plugin contract.
// It declares fs.read / fs.write permissions and contributes commands;
// native execution is bound by the host application.
package fs

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.fs"

// New returns the official filesystem plugin contribution.
func New() plugin.Plugin {
	return fsPlugin{}
}

type fsPlugin struct{}

func (fsPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Filesystem",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "Scoped filesystem access",
		Permissions: []domain.PermissionName{"fs.read", "fs.write"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (fsPlugin) Contribute() (plugin.Contribution, error) {
	read, err := domain.NewCommandDefinition("fs.read", "Read a file within grant scope", "fs.read")
	if err != nil {
		return plugin.Contribution{}, err
	}
	read.WithPlugin(PluginID)
	write, err := domain.NewCommandDefinition("fs.write", "Write a file within grant scope", "fs.write")
	if err != nil {
		return plugin.Contribution{}, err
	}
	write.WithPlugin(PluginID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{read, write},
		Events:   []domain.EventName{"fs.changed"},
	}, nil
}
