// Package app is the official Phase 3 application lifecycle plugin contract.
// It declares app.quit; hosts bind execution via desktop.AppService → App.Quit.
package app

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.app"

// New returns the official app plugin contribution.
func New() plugin.Plugin {
	return appPlugin{}
}

type appPlugin struct{}

func (appPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "App",
		Version:     plugin.SemVer{Major: 1},
		Description: "Application lifecycle commands",
		Permissions: []domain.PermissionName{"app.quit"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (appPlugin) Contribute() (plugin.Contribution, error) {
	quit, err := domain.NewCommandDefinition("app.quit", "Quit the application", "app.quit")
	if err != nil {
		return plugin.Contribution{}, err
	}
	quit.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{quit}}, nil
}
