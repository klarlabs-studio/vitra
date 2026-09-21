// Package clipboard is the official Phase 3 system clipboard plugin contract.
// It declares clipboard.read / clipboard.write permissions and contributes
// commands; native execution is bound by the host application.
package clipboard

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.clipboard"

// New returns the official clipboard plugin contribution.
func New() plugin.Plugin {
	return clipboardPlugin{}
}

type clipboardPlugin struct{}

func (clipboardPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Clipboard",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "System clipboard read/write",
		Permissions: []domain.PermissionName{"clipboard.read", "clipboard.write"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (clipboardPlugin) Contribute() (plugin.Contribution, error) {
	read, err := domain.NewCommandDefinition("clipboard.read", "Read the system clipboard", "clipboard.read")
	if err != nil {
		return plugin.Contribution{}, err
	}
	read.WithPlugin(PluginID)
	write, err := domain.NewCommandDefinition("clipboard.write", "Write the system clipboard", "clipboard.write")
	if err != nil {
		return plugin.Contribution{}, err
	}
	write.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{read, write}}, nil
}
