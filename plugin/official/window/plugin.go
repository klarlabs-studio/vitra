// Package window is the official Phase 3 multi-window plugin contract.
// It declares window.create / window.close; native execution is bound by the
// host via desktop.WindowService Create/Close wrapping app.App.
package window

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.window"

// New returns the official window plugin contribution.
func New() plugin.Plugin {
	return windowPlugin{}
}

type windowPlugin struct{}

func (windowPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Windows",
		Version:     plugin.SemVer{Major: 1},
		Description: "Create and close application windows",
		Permissions: []domain.PermissionName{"window.create", "window.close"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (windowPlugin) Contribute() (plugin.Contribution, error) {
	create, err := domain.NewCommandDefinition("window.create", "Open an application window", "window.create")
	if err != nil {
		return plugin.Contribution{}, err
	}
	create.WithPlugin(PluginID)
	closeCmd, err := domain.NewCommandDefinition("window.close", "Close an application window", "window.close")
	if err != nil {
		return plugin.Contribution{}, err
	}
	closeCmd.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{create, closeCmd}}, nil
}
