// Package window is the official Phase 3 multi-window plugin contract.
// It declares window.create / window.close / window.chrome / window.getChrome /
// window.focus / window.hide / window.show; native execution is bound by the
// host via desktop.WindowService wrapping app.App / DesktopHost.
package window

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// PluginID is the stable official plugin id.
const PluginID domain.PluginID = "vitra.window"

// New returns the official window plugin contribution.
func New() plugin.Plugin {
	return windowPlugin{}
}

type windowPlugin struct{}

func (windowPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          PluginID,
		Name:        "Windows",
		Version:     plugin.SemVer{Major: 1},
		Description: "Create, close, apply, read, focus, hide, and show application windows",
		Permissions: []domain.PermissionName{"window.create", "window.close", "window.chrome"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (windowPlugin) Contribute() (plugin.Contribution, error) {
	create, err := domain.NewCommandDefinition("window.create", "Open an application window", "window.create")
	if err != nil {
		return plugin.Contribution{}, err
	}
	create.WithPlugin(PluginID)
	closeCmd, err := domain.NewCommandDefinition("window.close", "Close an application window", "window.close")
	if err != nil {
		return plugin.Contribution{}, err
	}
	closeCmd.WithPlugin(PluginID)
	chrome, err := domain.NewCommandDefinition("window.chrome", "Apply window presentation (title, size, chrome)", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	chrome.WithPlugin(PluginID)
	getChrome, err := domain.NewCommandDefinition("window.getChrome", "Read window presentation (title, size, chrome)", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	getChrome.WithPlugin(PluginID)
	focus, err := domain.NewCommandDefinition("window.focus", "Raise and focus an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	focus.WithPlugin(PluginID)
	hide, err := domain.NewCommandDefinition("window.hide", "Hide an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	hide.WithPlugin(PluginID)
	show, err := domain.NewCommandDefinition("window.show", "Show an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	show.WithPlugin(PluginID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{create, closeCmd, chrome, getChrome, focus, hide, show}}, nil
}
