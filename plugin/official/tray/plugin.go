// Package tray is the official Phase 3 system-tray plugin contract.
// It declares tray.set and contributes tray.action; native execution is bound
// by the host via desktop.TrayService wrapping DesktopHost.SetTray.
package tray

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.tray"

// New returns the official tray plugin contribution.
func New() plugin.Plugin {
	return trayPlugin{}
}

type trayPlugin struct{}

func (trayPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Tray",
		Version:     plugin.SemVer{Major: 1},
		Description: "Set the system tray icon/menu and receive tray actions",
		Permissions: []domain.PermissionName{"tray.set"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (trayPlugin) Contribute() (plugin.Contribution, error) {
	set, err := domain.NewCommandDefinition("tray.set", "Set the system tray icon and context menu", "tray.set")
	if err != nil {
		return plugin.Contribution{}, err
	}
	set.WithPlugin(PluginID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{set},
		Events:   []domain.EventName{"tray.action"},
	}, nil
}
