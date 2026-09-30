package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// NotificationID is the stable id of the Notification plugin.
const NotificationID domain.PluginID = "vitra.notification"

// Notification returns the official notification plugin contribution.
func Notification() plugin.Plugin {
	return notificationPlugin{}
}

type notificationPlugin struct{}

func (notificationPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          NotificationID,
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
	show.WithPlugin(NotificationID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{show}}, nil
}
