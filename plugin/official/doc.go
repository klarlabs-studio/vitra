// Package official holds Vitra's official plugins: the permission and
// command contracts for the built-in desktop services (filesystem, dialogs,
// clipboard, windows, menus, …).
//
// A plugin only declares commands and permissions; the app binds executors
// (usually from package desktop) and grants the permissions it wants to
// allow. Registering a plugin grants nothing.
//
// The package also defines each command's input and output types
// (WindowSizeInput, DialogOptions, ...). app.UseOfficialPlugins binds the
// commands with them through vitra.Bind, so input is decoded strictly and
// the generated TypeScript client is typed; bind them the same way when
// binding a command by hand.
package official

import "go.klarlabs.de/vitra/plugin"

// All returns every official plugin, in a stable order.
func All() []plugin.Plugin {
	return []plugin.Plugin{
		FS(), Dialog(), Clipboard(), Browser(), OS(), Notification(), Path(),
		Window(), Menu(), Tray(), DragDrop(), Shortcut(), App(), DeepLink(),
	}
}
