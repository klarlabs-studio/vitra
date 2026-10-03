package platform

import "go.klarlabs.de/vitra/domain"

// MenuItem is a flat native menu-bar entry. Menu is the top-level label
// (e.g. "File"); ID is the action identifier delivered to SetActionHandler.
// Shortcut is an optional in-window accelerator (e.g. "Ctrl+Q" or
// "<Control>q"); it is not a global OS hotkey. A Separator item draws a
// divider and has no ID or Label. Disabled items are greyed out and never
// fire; Checked items show a checkmark.
type MenuItem struct {
	Menu      string
	ID        string
	Label     string
	Shortcut  string
	Separator bool
	Disabled  bool
	Checked   bool
}

// TraySpec is the tray (menu bar status item) a host shows.
type TraySpec struct {
	// Tooltip is shown when hovering the icon.
	Tooltip string
	// Title is text next to the icon: the macOS menu bar and
	// StatusNotifierItem labels. Hosts without tray text report
	// FeatureTrayTitle unavailable and show it in the tooltip instead.
	Title string
	// Icon is a PNG image; nil keeps the platform's default icon.
	Icon []byte
	// Template marks Icon as a macOS template image, tinted by the system
	// to match the menu bar. Other hosts ignore it.
	Template bool
	// Items is the tray menu.
	Items []MenuItem
	// ClickActivates makes a primary (left) click on the icon a TrayClick,
	// reported through TrayClickReporter, and opens the menu on a secondary
	// (right) click instead. Without it a click opens the menu on macOS and
	// reports the "tray.activate" action on Linux and Windows.
	ClickActivates bool
	// Panel names a panel window (WindowKindPanel) that a primary click
	// shows under the icon and hides again; it implies ClickActivates. The
	// app does this through Panels, so hosts ignore the field.
	Panel domain.WindowID
}

// Rect is a screen rectangle in the host's window coordinates: origin at the
// top-left of the primary display, in the units the host positions windows
// in. Pass it back to the same host; it is not portable between hosts.
type Rect struct {
	X, Y, Width, Height int
}

// TrayClick is a primary click on the tray icon (TraySpec.ClickActivates).
type TrayClick struct {
	// Anchor is the icon's rectangle when the host knows it. Some Linux
	// panels report only the click position (a zero-size Anchor) or nothing.
	Anchor Rect
	// HasAnchor is false when the host could not tell where the icon is.
	HasAnchor bool
}

// TooltipWithTitle is the tooltip of a tray that cannot show text next to
// its icon: the title joins the tooltip, on its own line.
func (s TraySpec) TooltipWithTitle() string {
	switch {
	case s.Title == "":
		return s.Tooltip
	case s.Tooltip == "":
		return s.Title
	default:
		return s.Title + "\n" + s.Tooltip
	}
}
