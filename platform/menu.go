package platform

// MenuItem is a flat native menu-bar entry. Menu is the top-level label
// (e.g. "File"); ID is the action identifier delivered to SetActionHandler.
// Shortcut is an optional in-window accelerator (e.g. "Ctrl+Q" or
// "<Control>q"); it is not a global OS hotkey.
type MenuItem struct {
	Menu     string
	ID       string
	Label    string
	Shortcut string
}
