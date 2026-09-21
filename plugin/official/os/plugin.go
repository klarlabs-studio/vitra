// Package os is the official Phase 3 host-info plugin contract.
// It declares os.info permission and contributes a command; execution is
// bound by the host application via desktop.OsService (stdlib fill-in).
package os

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.os"

// New returns the official os plugin contribution.
func New() plugin.Plugin {
	return osPlugin{}
}

type osPlugin struct{}

func (osPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "OS",
		Version:     plugin.SemVer{Major: 1, Minor: 0, Patch: 0},
		Description: "Read-only host platform information",
		Permissions: []domain.PermissionName{"os.info"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3, Patch: 0},
	}
}

func (osPlugin) Contribute() (plugin.Contribution, error) {
	info, err := domain.NewCommandDefinition("os.info", "Read host OS/arch/locale", "os.info")
	if err != nil {
		return plugin.Contribution{}, err
	}
	info.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{info}}, nil
}
