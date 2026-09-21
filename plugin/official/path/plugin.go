// Package path is the official Phase 3 local-path opener plugin contract.
// It declares path.open permission and contributes a command; native
// execution is bound by the host application via desktop.PathService.
package path

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.path"

// New returns the official path plugin contribution.
func New() plugin.Plugin {
	return pathPlugin{}
}

type pathPlugin struct{}

func (pathPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
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
	open.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open}}, nil
}
