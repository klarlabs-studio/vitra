// Package deeplink is the official Phase 3 deep-link plugin contract.
// It owns deeplink.handle and contributes deeplink.open; native hosts accept
// links via DesktopHost.StartDeepLinkBridge / argv and emit after
// desktop.DeepLinkService.Handle succeeds (patterns stay host-configured).
package deeplink

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.deeplink"

// New returns the official deep-link plugin contribution.
func New() plugin.Plugin {
	return deeplinkPlugin{}
}

type deeplinkPlugin struct{}

func (deeplinkPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
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
