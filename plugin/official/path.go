package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PathID is the stable id of the Path plugin.
const PathID domain.PluginID = "vitra.path"

// Path returns the official path plugin contribution.
func Path() plugin.Plugin {
	return pathPlugin{}
}

type pathPlugin struct{}

func (pathPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PathID,
		Name:        "Path",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "Open local filesystem paths with the OS default handler",
		Permissions: []domain.PermissionName{"path.open"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (pathPlugin) Contribute() (plugin.Contribution, error) {
	open, err := domain.NewCommandDefinition("path.open", "Open a local path with the OS default handler", "path.open")
	if err != nil {
		return plugin.Contribution{}, err
	}
	open.WithPlugin(PathID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open}}, nil
}
