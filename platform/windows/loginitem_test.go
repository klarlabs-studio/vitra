//go:build windows

package windows

import (
	"os"
	"testing"
)

// Launch at login is a quoted value under HKCU\...\Run.
func TestLoginItem(t *testing.T) {
	h := &Host{}
	const id = "de.klarlabs.vitra.test-login-item"
	t.Cleanup(func() { _ = h.SetLoginItem(id, "", false) })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SetLoginItem(id, exe, true); err != nil {
		t.Fatal(err)
	}
	if on, err := h.LoginItemEnabled(id); err != nil || !on {
		t.Fatalf("after enabling: %v, %v", on, err)
	}
	if v, err := runValue(id); err != nil || v != `"`+exe+`"` {
		t.Fatalf("Run value %q, %v", v, err)
	}
	if err := h.SetLoginItem(id, "", false); err != nil {
		t.Fatal(err)
	}
	if on, err := h.LoginItemEnabled(id); err != nil || on {
		t.Fatalf("after disabling: %v, %v", on, err)
	}
	if err := h.SetLoginItem(id, "", false); err != nil {
		t.Fatalf("disabling twice: %v", err)
	}
}
