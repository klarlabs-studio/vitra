package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// BrowserID is the stable id of the Browser plugin.
const BrowserID domain.PluginID = "vitra.browser"

// Browser returns the official browser plugin contribution.
func Browser() plugin.Plugin {
	return browserPlugin{}
}

type browserPlugin struct{}

func (browserPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          BrowserID,
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
	open.WithPlugin(BrowserID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{open}}, nil
}
