// Package shortcut is the official Phase 3 global-shortcut plugin contract.
// It declares shortcut.register / shortcut.unregister and contributes
// shortcut.action; native execution is bound via desktop.ShortcutService →
// RegisterGlobalShortcut / UnregisterGlobalShortcut.
// Wayland hosts return ErrUnsupported (no portable global hotkey API).
package shortcut

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.shortcut"

// New returns the official shortcut plugin contribution.
func New() plugin.Plugin {
	return shortcutPlugin{}
}

type shortcutPlugin struct{}

func (shortcutPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
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
	reg.WithPlugin(PluginID)
	unreg, err := domain.NewCommandDefinition("shortcut.unregister", "Unregister a global OS shortcut", "shortcut.register")
	if err != nil {
		return plugin.Contribution{}, err
	}
	unreg.WithPlugin(PluginID)
	return plugin.Contribution{
		Commands: []*domain.CommandDefinition{reg, unreg},
		Events:   []domain.EventName{"shortcut.action"},
	}, nil
}
