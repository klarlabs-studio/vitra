//go:build !windows

package windows

import "go.klarlabs.de/vitra/platform"

// LoginItemEnabled returns ErrUnsupported: login items need Windows.
func (h *Host) LoginItemEnabled(string) (bool, error) {
	return false, &platform.ErrUnsupported{Feature: platform.FeatureLoginItem, OS: platform.OSWindows, Detail: "requires Windows"}
}

// SetLoginItem returns ErrUnsupported: login items need Windows.
func (h *Host) SetLoginItem(string, string, bool) error {
	return &platform.ErrUnsupported{Feature: platform.FeatureLoginItem, OS: platform.OSWindows, Detail: "requires Windows"}
}
