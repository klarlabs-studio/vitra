package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// MenuID is the stable id of the Menu plugin.
const MenuID domain.PluginID = "vitra.menu"

// Menu returns the official menu plugin contribution.
func Menu() plugin.Plugin {
	return menuPlugin{}
}

type menuPlugin struct{}

func (menuPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          MenuID,
		Name:        "Menu",
		Version:     plugin.SemVer{Major: 1},
		Description: "Set or clear the application menu bar and receive menu actions",
		Permissions: []domain.PermissionName{"menu.set"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (menuPlugin) Contribute() (plugin.Contribution, error) {
	set, err := domain.NewCommandDefinition("menu.set", "Replace the application menu bar", "menu.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	set.WithPlugin(MenuID)
	clear, err := domain.NewCommandDefinition("menu.clear", "Clear the application menu bar", "menu.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	clear.WithPlugin(MenuID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{set, clear},
		Events:   []domain.EventName{"menu.action"},
	}, nil
}
