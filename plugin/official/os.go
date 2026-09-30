package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// OSID is the stable id of the OS plugin.
const OSID domain.PluginID = "vitra.os"

// OS returns the official os plugin contribution.
func OS() plugin.Plugin {
	return osPlugin{}
}

type osPlugin struct{}

func (osPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          OSID,
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
	info.WithPlugin(OSID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{info}}, nil
}
