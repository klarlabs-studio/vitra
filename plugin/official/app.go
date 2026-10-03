package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// AppID is the stable id of the App plugin.
const AppID domain.PluginID = "vitra.app"

// App returns the official app plugin contribution.
func App() plugin.Plugin {
	return appPlugin{}
}

type appPlugin struct{}

func (appPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          AppID,
		Name:        "App",
		Version:     plugin.SemVer{Major: 1},
		Description: "Application lifecycle commands: quit, launch at login",
		Permissions: []domain.PermissionName{"app.quit", "app.login_item"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (appPlugin) Contribute() (plugin.Contribution, error) {
	quit, err := domain.NewCommandDefinition("app.quit", "Quit the application", "app.quit")
	if err != nil {
		return plugin.Contribution{}, err
	}
	quit.WithPlugin(AppID)
	get, err := domain.NewCommandDefinition("app.loginItem", "Report whether the app starts at login", "app.login_item")
	if err != nil {
		return plugin.Contribution{}, err
	}
	get.WithPlugin(AppID)
	set, err := domain.NewCommandDefinition("app.setLoginItem", "Start the app at login, or stop it", "app.login_item")
	if err != nil {
		return plugin.Contribution{}, err
	}
	set.WithPlugin(AppID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{quit, get, set}}, nil
}
