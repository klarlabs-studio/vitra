// Package browser is the official Phase 3 system browser plugin contract.
// It declares browser.open permission and contributes a command; native
// execution is bound by the host application via desktop.BrowserService.
package browser

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.browser"

// New returns the official browser plugin contribution.
func New() plugin.Plugin {
	return browserPlugin{}
}

type browserPlugin struct{}

func (browserPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Browser",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "Open URLs in the system default browser",
		Permissions: []domain.PermissionName{"browser.open"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (browserPlugin) Contribute() (plugin.Contribution, error) {
	open, err := domain.NewCommandDefinition("browser.open", "Open a URL in the system browser", "browser.open")
	if err != nil {
		return plugin.Contribution{}, err
	}
	open.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open}}, nil
}
