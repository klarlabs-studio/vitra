package app

import "go.klarlabs.de/vitra/platform"

// applyPresentation hands an accessory presentation to the host. A regular
// app leaves the host's default alone, so hosts without the capability keep
// working.
func (a *App) applyPresentation() error {
	if !a.opts.accessory() {
		return nil
	}
	if err := platform.Require(a.host, platform.FeaturePresentation); err != nil {
		return err
	}
	h, ok := a.host.(platform.PresentationSetter)
	if !ok {
		return &platform.ErrUnsupported{Feature: platform.FeaturePresentation, OS: a.host.OS(), Detail: "host cannot change the app's presentation"}
	}
	return h.SetPresentation(platform.PresentationAccessory)
}
