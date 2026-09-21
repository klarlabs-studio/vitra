// Package menu is the official Phase 3 application menu-bar plugin contract.
// It declares menu.set and contributes menu.action; native execution is bound
// by the host via desktop.MenuService wrapping DesktopHost.SetMenuBar.
package menu

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.menu"

// New returns the official menu plugin contribution.
func New() plugin.Plugin {
	return menuPlugin{}
}

type menuPlugin struct{}

func (menuPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Menu",
		Version:     plugin.SemVer{Major: 1},
		Description: "Set the application menu bar and receive menu actions",
		Permissions: []domain.PermissionName{"menu.set"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (menuPlugin) Contribute() (plugin.Contribution, error) {
	set, err := domain.NewCommandDefinition("menu.set", "Replace the application menu bar", "menu.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	set.WithPlugin(PluginID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{set},
		Events:   []domain.EventName{"menu.action"},
	}, nil
}
