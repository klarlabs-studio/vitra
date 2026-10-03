package desktop_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// A service whose native hook is not bound (the host lacks the optional
// capability) fails explicitly, even when the feature matrix claims support.
// It never reports success for work it did not do (security invariant 14).
func TestUnboundHooks_FailUnsupported(t *testing.T) {
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	host := withFeatures(platform.OSLinux, platform.FeatureMenuBar, platform.FeatureTray)
	ctx := context.Background()
	menus := &desktop.MenuService{Gateway: allowAll{}, Host: host}
	trays := &desktop.TrayService{Gateway: allowAll{}, Host: host}
	items := []desktop.MenuItem{{ID: "quit", Label: "Quit"}}

	for _, c := range []struct {
		name    string
		err     error
		feature platform.Feature
	}{
		{"menu.set", menus.SetMenu(ctx, caller, items), platform.FeatureMenuBar},
		{"menu.clear", menus.ClearMenu(ctx, caller), platform.FeatureMenuBar},
		{"tray.set", trays.SetTray(ctx, caller, desktop.TraySpec{Tooltip: "tip", Items: items}), platform.FeatureTray},
		{"tray.clear", trays.ClearTray(ctx, caller), platform.FeatureTray},
	} {
		var un *platform.ErrUnsupported
		if !errors.As(c.err, &un) || un.Feature != c.feature {
			t.Errorf("%s without a hook = %v, want *platform.ErrUnsupported for %s", c.name, c.err, c.feature)
		}
	}
}
