//go:build linux

package linux

import "testing"

func TestPortalTrigger(t *testing.T) {
	ok := []struct{ in, want string }{
		{"Ctrl+Shift+Y", "CTRL+SHIFT+y"},
		{"shift+ctrl+y", "CTRL+SHIFT+y"},
		{"CmdOrCtrl+K", "CTRL+k"},
		{"Alt+Space", "ALT+space"},
		{"Super+F5", "LOGO+F5"},
		{"Meta+Alt+1", "ALT+LOGO+1"},
		{"Ctrl+Alt+Delete", "CTRL+ALT+Delete"},
		{"Ctrl+PageDown", "CTRL+Page_Down"},
		{"Ctrl++", "CTRL+plus"},
		{"Ctrl+-", "CTRL+minus"},
		{"Ctrl+F12", "CTRL+F12"},
		{"<Control><Shift>y", "CTRL+SHIFT+y"},
		{"<Super>Return", "LOGO+Return"},
	}
	for _, tc := range ok {
		got, err := portalTrigger(tc.in)
		if err != nil {
			t.Errorf("portalTrigger(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("portalTrigger(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	bad := []string{
		"", "y", "F5", // no modifier
		"Ctrl+", "Ctrl+Shift", // no key / modifier as key
		"Hyperspace+y",                   // unknown modifier
		"Ctrl+F25", "Ctrl+Foo", "Ctrl+é", // unknown keys
		"<Control", // unclosed
		"Ctrl++Y",  // empty modifier
	}
	for _, in := range bad {
		if got, err := portalTrigger(in); err == nil {
			t.Errorf("portalTrigger(%q) = %q, want error", in, got)
		}
	}
}
