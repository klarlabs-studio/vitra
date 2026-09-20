package platform

// MenuItem is a flat native menu-bar entry. Menu is the top-level label
// (e.g. "File"); ID is the action identifier delivered to SetActionHandler.
type MenuItem struct {
	Menu  string
	ID    string
	Label string
}
