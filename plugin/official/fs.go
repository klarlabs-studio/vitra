package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// FSID is the stable id of the FS plugin.
const FSID domain.PluginID = "vitra.fs"

// FS returns the official filesystem plugin contribution.
func FS() plugin.Plugin {
	return fsPlugin{}
}

type fsPlugin struct{}

func (fsPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          FSID,
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
	read.WithPlugin(FSID)
	write, err := domain.NewCommandDefinition("fs.write", "Write a file within grant scope", "fs.write")
	if err != nil {
		return plugin.Contribution{}, err
	}
	write.WithPlugin(FSID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{read, write},
		Events:   []domain.EventName{"fs.changed"},
	}, nil
}
