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
	quit.WithPlugin(AppID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{quit}}, nil
}
