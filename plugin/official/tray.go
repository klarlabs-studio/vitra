package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// TrayID is the stable id of the Tray plugin.
const TrayID domain.PluginID = "vitra.tray"

// Tray returns the official tray plugin contribution.
func Tray() plugin.Plugin {
	return trayPlugin{}
}

type trayPlugin struct{}

func (trayPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          TrayID,
		Name:        "Tray",
		Version:     plugin.SemVer{Major: 1},
		Description: "Set or clear the system tray icon/menu and receive tray actions and clicks",
		Permissions: []domain.PermissionName{"tray.set"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (trayPlugin) Contribute() (plugin.Contribution, error) {
	set, err := domain.NewCommandDefinition("tray.set", "Set the system tray icon and context menu", "tray.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	set.WithPlugin(TrayID)
	clear, err := domain.NewCommandDefinition("tray.clear", "Clear the system tray icon", "tray.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	clear.WithPlugin(TrayID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{set, clear},
		Events:   []domain.EventName{"tray.action", "tray.click"},
	}, nil
}
