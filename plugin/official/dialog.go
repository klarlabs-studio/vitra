package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// DialogID is the stable id of the Dialog plugin.
const DialogID domain.PluginID = "vitra.dialog"

// Dialog returns the official dialog plugin contribution.
func Dialog() plugin.Plugin {
	return dialogPlugin{}
}

type dialogPlugin struct{}

func (dialogPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          DialogID,
		Name:        "Dialogs",
		Version:     plugin.SemVer{Major: 1},
		Description: "Native open/save/directory/message dialogs (open/save/directory accept title, defaultPath; open/save also filters)",
		Permissions: []domain.PermissionName{"dialog.open", "dialog.save", "dialog.openDirectory", "dialog.message"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (dialogPlugin) Contribute() (plugin.Contribution, error) {
	open, err := domain.NewCommandDefinition("dialog.open", "Open file dialog", "dialog.open")
	if err != nil {
		return plugin.Contribution{}, err
	}
	open.WithPlugin(DialogID)
	save, err := domain.NewCommandDefinition("dialog.save", "Save file dialog", "dialog.save")
	if err != nil {
		return plugin.Contribution{}, err
	}
	save.WithPlugin(DialogID)
	opendir, err := domain.NewCommandDefinition("dialog.openDirectory", "Open directory dialog (optional title, defaultPath)", "dialog.openDirectory")
	if err != nil {
		return plugin.Contribution{}, err
	}
	opendir.WithPlugin(DialogID)
	msg, err := domain.NewCommandDefinition("dialog.message", "Show a native message or confirm dialog", "dialog.message")
	if err != nil {
		return plugin.Contribution{}, err
	}
	msg.WithPlugin(DialogID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open, save, opendir, msg}}, nil
}
