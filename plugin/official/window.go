package official

import (
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
)

// WindowID is the stable id of the Window plugin.
const WindowID domain.PluginID = "vitra.window"

// Window returns the official window plugin contribution.
func Window() plugin.Plugin {
	return windowPlugin{}
}

type windowPlugin struct{}

func (windowPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          WindowID,
		Name:        "Windows",
		Version:     plugin.SemVer{Major: 1},
		Description: "Create, close, apply, read, focus, blur, hide, show, minimize, maximize, unmaximize, fullscreen, unfullscreen, always-on-top, restore, set title, set size, and set icon for application windows",
		Permissions: []domain.PermissionName{"window.create", "window.close", "window.chrome"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (windowPlugin) Contribute() (plugin.Contribution, error) {
	create, err := domain.NewCommandDefinition("window.create", "Open an application window", "window.create")
	if err != nil {
		return plugin.Contribution{}, err
	}
	create.WithPlugin(WindowID)
	closeCmd, err := domain.NewCommandDefinition("window.close", "Close an application window", "window.close")
	if err != nil {
		return plugin.Contribution{}, err
	}
	closeCmd.WithPlugin(WindowID)
	chrome, err := domain.NewCommandDefinition("window.chrome", "Apply window presentation (title, size, chrome)", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	chrome.WithPlugin(WindowID)
	getChrome, err := domain.NewCommandDefinition("window.getChrome", "Read window presentation (title, size, chrome)", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	getChrome.WithPlugin(WindowID)
	focus, err := domain.NewCommandDefinition("window.focus", "Raise and focus an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	focus.WithPlugin(WindowID)
	blur, err := domain.NewCommandDefinition("window.blur", "Resign key focus on an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	blur.WithPlugin(WindowID)
	hide, err := domain.NewCommandDefinition("window.hide", "Hide an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	hide.WithPlugin(WindowID)
	show, err := domain.NewCommandDefinition("window.show", "Show an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	show.WithPlugin(WindowID)
	minimize, err := domain.NewCommandDefinition("window.minimize", "Minimize an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	minimize.WithPlugin(WindowID)
	maximize, err := domain.NewCommandDefinition("window.maximize", "Maximize an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	maximize.WithPlugin(WindowID)
	unmaximize, err := domain.NewCommandDefinition("window.unmaximize", "Restore an application window from maximized", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	unmaximize.WithPlugin(WindowID)
	fullscreen, err := domain.NewCommandDefinition("window.fullscreen", "Enter fullscreen for an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	fullscreen.WithPlugin(WindowID)
	unfullscreen, err := domain.NewCommandDefinition("window.unfullscreen", "Exit fullscreen for an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	unfullscreen.WithPlugin(WindowID)
	alwaysOnTop, err := domain.NewCommandDefinition("window.setAlwaysOnTop", "Toggle always-on-top for an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	alwaysOnTop.WithPlugin(WindowID)
	restore, err := domain.NewCommandDefinition("window.restore", "Restore an application window from minimized/maximized/fullscreen", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	restore.WithPlugin(WindowID)
	setTitle, err := domain.NewCommandDefinition("window.setTitle", "Set the title of an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	setTitle.WithPlugin(WindowID)
	setSize, err := domain.NewCommandDefinition("window.setSize", "Set the size of an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	setSize.WithPlugin(WindowID)
	setIcon, err := domain.NewCommandDefinition("window.setIcon", "Set the icon of an application window", "window.chrome")
	if err != nil {
		return plugin.Contribution{}, err
	}
	setIcon.WithPlugin(WindowID)
	return plugin.Contribution{Commands: []*domain.CommandDefinition{create, closeCmd, chrome, getChrome, focus, blur, hide, show, minimize, maximize, unmaximize, fullscreen, unfullscreen, alwaysOnTop, restore, setTitle, setSize, setIcon}}, nil
}
