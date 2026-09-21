// Package notification is the official Phase 3 desktop notification plugin contract.
// It declares notifications.show permission and contributes a command; native
// execution is bound by the host application via desktop.NotificationService.
package notification

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.notification"

// New returns the official notification plugin contribution.
func New() plugin.Plugin {
	return notificationPlugin{}
}

type notificationPlugin struct{}

func (notificationPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Notification",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "Show desktop notifications",
		Permissions: []domain.PermissionName{"notifications.show"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (notificationPlugin) Contribute() (plugin.Contribution, error) {
	show, err := domain.NewCommandDefinition("notifications.show", "Show a desktop notification", "notifications.show")
	if err != nil {
		return plugin.Contribution{}, err
	}
	show.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{show}}, nil
}
