package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// ClipboardID is the stable id of the Clipboard plugin.
const ClipboardID domain.PluginID = "vitra.clipboard"

// Clipboard returns the official clipboard plugin contribution.
func Clipboard() plugin.Plugin {
	return clipboardPlugin{}
}

type clipboardPlugin struct{}

func (clipboardPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ClipboardID,
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
	read.WithPlugin(ClipboardID)
	write, err := domain.NewCommandDefinition("clipboard.write", "Write the system clipboard", "clipboard.write")
	if err != nil {
		return plugin.Contribution{}, err
	}
	write.WithPlugin(ClipboardID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{read, write}}, nil
}
