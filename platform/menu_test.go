package platform_test

import (
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestTraySpec_TooltipWithTitle(t *testing.T) {
	for _, c := range []struct{ title, tooltip, want string }{
		{"", "", ""}, {"", "tip", "tip"}, {"42%", "", "42%"}, {"42%", "Usage", "42%\nUsage"},
	} {
		if got := (platform.TraySpec{Title: c.title, Tooltip: c.tooltip}).TooltipWithTitle(); got != c.want {
			t.Errorf("TooltipWithTitle(%q, %q) = %q, want %q", c.title, c.tooltip, got, c.want)
		}
	}
}
