package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// DragDropID is the stable id of the DragDrop plugin.
const DragDropID domain.PluginID = "vitra.dragdrop"

// DragDrop returns the official drag-drop plugin contribution.
func DragDrop() plugin.Plugin {
	return dragdropPlugin{}
}

type dragdropPlugin struct{}

func (dragdropPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          DragDropID,
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
	recv.WithPlugin(DragDropID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{recv},
		Events:   []domain.EventName{"dragdrop.drop"},
	}, nil
}
