package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// ShortcutID is the stable id of the Shortcut plugin.
const ShortcutID domain.PluginID = "vitra.shortcut"

// Shortcut returns the official shortcut plugin contribution.
func Shortcut() plugin.Plugin {
	return shortcutPlugin{}
}

type shortcutPlugin struct{}

func (shortcutPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ShortcutID,
		Name:        "Shortcut",
		Version:     plugin.SemVer{Major: 1},
		Description: "Register or unregister OS-wide global shortcuts (unsupported on Wayland)",
		Permissions: []domain.PermissionName{"shortcut.register"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (shortcutPlugin) Contribute() (plugin.Contribution, error) {
	reg, err := domain.NewCommandDefinition("shortcut.register", "Register a global OS shortcut", "shortcut.register")
	if err != nil {
		return plugin.Contribution{}, err
	}
	reg.WithPlugin(ShortcutID)
	unreg, err := domain.NewCommandDefinition("shortcut.unregister", "Unregister a global OS shortcut", "shortcut.register")
	if err != nil {
		return plugin.Contribution{}, err
	}
	unreg.WithPlugin(ShortcutID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{reg, unreg},
		Events:   []domain.EventName{"shortcut.action"},
	}, nil
}
