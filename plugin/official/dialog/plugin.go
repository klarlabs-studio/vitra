// Package dialog is the official Phase 3 native dialog plugin contract.
package dialog

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.dialog"

// New returns the official dialog plugin contribution.
func New() plugin.Plugin {
	return dialogPlugin{}
}

type dialogPlugin struct{}

func (dialogPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Dialogs",
		Version:     plugin.SemVer{Major: 1},
		Description: "Native open/save dialogs",
		Permissions: []domain.PermissionName{"dialog.open", "dialog.save"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (dialogPlugin) Contribute() (plugin.Contribution, error) {
	open, err := domain.NewCommandDefinition("dialog.open", "Open file dialog", "dialog.open")
	if err != nil {
		return plugin.Contribution{}, err
	}
	open.WithPlugin(PluginID)
	save, err := domain.NewCommandDefinition("dialog.save", "Save file dialog", "dialog.save")
	if err != nil {
		return plugin.Contribution{}, err
	}
	save.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open, save}}, nil
}
