package app

import (
	"os"
	"path/filepath"

	"go.klarlabs.de/vitra/platform"
)

// LoginItemEnabled reports whether the app starts with the user's session.
// It fails with *platform.ErrUnsupported where the host cannot tell, as for
// an unbundled macOS app.
func (a *App) LoginItemEnabled() (bool, error) {
	h, err := a.loginItems()
	if err != nil {
		return false, err
	}
	return h.LoginItemEnabled(string(a.opts.AppID))
}

// SetLoginItem makes the app start with the user's session, or stops it.
// It registers the running executable (macOS registers the app bundle). On
// macOS the user may still have to approve it in System Settings.
func (a *App) SetLoginItem(enabled bool) error {
	h, err := a.loginItems()
	if err != nil {
		return err
	}
	exe := ""
	if enabled {
		if exe, err = os.Executable(); err != nil {
			return err
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
	}
	return h.SetLoginItem(string(a.opts.AppID), exe, enabled)
}

func (a *App) loginItems() (platform.LoginItems, error) {
	if err := platform.Require(a.host, platform.FeatureLoginItem); err != nil {
		return nil, err
	}
	h, ok := a.host.(platform.LoginItems)
	if !ok {
		return nil, &platform.ErrUnsupported{Feature: platform.FeatureLoginItem, OS: a.host.OS(), Detail: "host cannot register login items"}
	}
	return h, nil
}
