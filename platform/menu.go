package platform

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
