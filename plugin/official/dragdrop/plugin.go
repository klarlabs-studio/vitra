// Package dragdrop is the official Phase 3 file drag-and-drop plugin contract.
// It declares dragdrop.receive and contributes dragdrop.drop; native execution
// is bound by the host via desktop.DragDropService wrapping EnableDragDrop.
package dragdrop

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.dragdrop"

// New returns the official drag-drop plugin contribution.
func New() plugin.Plugin {
	return dragdropPlugin{}
}

type dragdropPlugin struct{}

func (dragdropPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Drag and Drop",
		Version:     plugin.SemVer{Major: 1},
		Description: "Enable file drops on windows and receive drop events",
		Permissions: []domain.PermissionName{"dragdrop.receive"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (dragdropPlugin) Contribute() (plugin.Contribution, error) {
	recv, err := domain.NewCommandDefinition("dragdrop.receive", "Enable or disable file drops on a window", "dragdrop.receive")
	if err != nil {
		return plugin.Contribution{}, err
	}
	recv.WithPlugin(PluginID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{recv},
		Events:   []domain.EventName{"dragdrop.drop"},
	}, nil
}
