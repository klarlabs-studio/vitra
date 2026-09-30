package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// DeepLinkID is the stable id of the DeepLink plugin.
const DeepLinkID domain.PluginID = "vitra.deeplink"

// DeepLink returns the official deep-link plugin contribution.
func DeepLink() plugin.Plugin {
	return deeplinkPlugin{}
}

type deeplinkPlugin struct{}

func (deeplinkPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          DeepLinkID,
		Name:        "Deep Link",
		Version:     plugin.SemVer{Major: 1},
		Description: "Receive OS deep links matching host-configured patterns",
		Permissions: []domain.PermissionName{"deeplink.handle"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (deeplinkPlugin) Contribute() (plugin.Contribution, error) {
	return plugin.Contribution{
		Events: []domain.EventName{"deeplink.open"},
	}, nil
}
