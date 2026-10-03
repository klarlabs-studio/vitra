package darwin

import "go.klarlabs.de/vitra/platform"

// The host implements the core and every optional capability in package
// platform, with or without -tags vitra_native. Whether a feature works on
// this machine is reported by Features; without the tag every call fails
// with *platform.ErrUnsupported.
var (
	_ platform.DesktopHost              = (*Host)(nil)
	_ platform.DevToolsSetter           = (*Host)(nil)
	_ platform.ActionReporter           = (*Host)(nil)
	_ platform.Clipboard                = (*Host)(nil)
	_ platform.Dialogs                  = (*Host)(nil)
	_ platform.MultiFileOpener          = (*Host)(nil)
	_ platform.Notifier                 = (*Host)(nil)
	_ platform.MenuBar                  = (*Host)(nil)
	_ platform.Tray                     = (*Host)(nil)
	_ platform.PresentationSetter       = (*Host)(nil)
	_ platform.TrayClickReporter        = (*Host)(nil)
	_ platform.TrayAnchorer             = (*Host)(nil)
	_ platform.Panels                   = (*Host)(nil)
	_ platform.GlobalShortcuts          = (*Host)(nil)
	_ platform.DragDrop                 = (*Host)(nil)
	_ platform.WindowControls           = (*Host)(nil)
	_ platform.URLOpener                = (*Host)(nil)
	_ platform.PathOpener               = (*Host)(nil)
	_ platform.SingleInstance           = (*Host)(nil)
	_ platform.URLSchemeRegistrar       = (*Host)(nil)
	_ platform.FileAssociationRegistrar = (*Host)(nil)
	_ platform.ScriptEvaluator          = (*Host)(nil)
	_ platform.FileDropInjector         = (*Host)(nil)
)
