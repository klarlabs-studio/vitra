//go:build linux && cgo && vitra_native

package linux

import (
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// These tests drive the Wayland GlobalShortcuts path against a fake portal
// (testdata/fake_portal.py) on a private session bus. forcePortal makes the
// host take that path under xvfb's X11 display.

func newPortalHost(t *testing.T) (*Host, chan string) {
	t.Helper()
	h := newHostOnThisThread()
	h.forcePortal = true
	got := make(chan string, 8)
	h.SetActionHandler(func(id string) { got <- id })
	return h, got
}

func expectAction(t *testing.T, got chan string, want string) {
	t.Helper()
	select {
	case id := <-got:
		if id != want {
			t.Fatalf("action %q, want %q", id, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("action %q never arrived", want)
	}
}

func expectNoAction(t *testing.T, got chan string) {
	t.Helper()
	select {
	case id := <-got:
		t.Fatalf("unexpected action %q", id)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestPortalShortcutsBindAndActivate(t *testing.T) {
	portal := startFake(t, "fake_portal.py", "grant")
	h, got := newPortalHost(t)

	if !h.Features().Available(platform.FeatureGlobalShortcut) {
		t.Fatalf("portal present: want FeatureGlobalShortcut, got %+v", h.Features()[platform.FeatureGlobalShortcut])
	}

	if err := h.RegisterGlobalShortcut("Ctrl+Shift+Y", "app.palette"); err != nil {
		t.Fatal(err)
	}
	portal.waitCall("CreateSession")
	bind := portal.waitCall("BindShortcuts")
	assertBound(t, bind, [][2]string{{"app.palette", "CTRL+SHIFT+y"}})

	// Activated on our session reaches the action handler (→ shortcut.action).
	portal.send("activate app.palette")
	expectAction(t, got, "app.palette")

	// Activations on a session that is not ours are ignored.
	portal.send("activate-foreign app.palette")
	portal.waitFor("foreign Activated", func(ev map[string]any) bool {
		s, _ := ev["session"].(string)
		return ev["emitted"] == "Activated" && s != "" && s == "/org/freedesktop/portal/desktop/session/someone_else/stolen"
	})
	expectNoAction(t, got)

	// A second shortcut rebinds the whole set on a new session and closes
	// the old one.
	if err := h.RegisterGlobalShortcut("Super+F5", "app.reload"); err != nil {
		t.Fatal(err)
	}
	portal.waitCall("CreateSession")
	bind = portal.waitCall("BindShortcuts")
	assertBound(t, bind, [][2]string{{"app.palette", "CTRL+SHIFT+y"}, {"app.reload", "LOGO+F5"}})
	portal.waitCall("Session.Close")
	portal.send("activate app.reload")
	expectAction(t, got, "app.reload")

	// Unregister rebinds what remains (the portal has no unbind).
	if err := h.UnregisterGlobalShortcut("Ctrl+Shift+Y"); err != nil {
		t.Fatal(err)
	}
	portal.waitCall("CreateSession")
	bind = portal.waitCall("BindShortcuts")
	assertBound(t, bind, [][2]string{{"app.reload", "LOGO+F5"}})
	portal.waitCall("Session.Close")
	if err := h.UnregisterGlobalShortcut("Ctrl+Shift+Y"); err == nil {
		t.Fatal("unregistering twice: want not-found")
	}

	// Removing the last shortcut closes the session.
	if err := h.UnregisterGlobalShortcut("Super+F5"); err != nil {
		t.Fatal(err)
	}
	portal.waitCall("Session.Close")
	portal.send("activate app.reload")
	expectNoAction(t, got)

	// Accelerators the portal cannot express are validation errors.
	var verr *domain.ErrValidation
	if err := h.RegisterGlobalShortcut("y", "app.bare"); !errors.As(err, &verr) {
		t.Fatalf("bare key: want ErrValidation, got %v", err)
	}
}

func TestPortalShortcutsRefused(t *testing.T) {
	for _, mode := range []string{"refuse", "cancel-session"} {
		t.Run(mode, func(t *testing.T) {
			startFake(t, "fake_portal.py", mode)
			h, _ := newPortalHost(t)
			err := h.RegisterGlobalShortcut("Ctrl+Alt+K", "app.k")
			if err == nil {
				t.Fatal("refused binding: want an error")
			}
			var unsupp *platform.ErrUnsupported
			if errors.As(err, &unsupp) {
				t.Fatalf("refusal must not read as unsupported: %v", err)
			}
			// Nothing was bound, so there is nothing to unregister.
			if err := h.UnregisterGlobalShortcut("Ctrl+Alt+K"); err == nil {
				t.Fatal("want not-found after a refused registration")
			}
		})
	}
}

func TestPortalShortcutsUnavailable(t *testing.T) {
	requireDBusFakes(t)
	// No fake running: nobody owns org.freedesktop.portal.Desktop.
	h, _ := newPortalHost(t)
	if h.Features().Available(platform.FeatureGlobalShortcut) {
		t.Fatal("no portal: FeatureGlobalShortcut must be unavailable")
	}
	var unsupp *platform.ErrUnsupported
	if err := h.RegisterGlobalShortcut("Ctrl+Shift+Y", "app.palette"); !errors.As(err, &unsupp) {
		t.Fatalf("no portal: want ErrUnsupported, got %v", err)
	}
	if unsupp.Feature != platform.FeatureGlobalShortcut {
		t.Fatalf("unsupported feature %q", unsupp.Feature)
	}
}

func assertBound(t *testing.T, ev map[string]any, want [][2]string) {
	t.Helper()
	raw, _ := ev["shortcuts"].([]any)
	if len(raw) != len(want) {
		t.Fatalf("bound %v, want %v", raw, want)
	}
	for i, w := range want {
		s, _ := raw[i].([]any)
		if len(s) < 2 || s[0] != w[0] || s[1] != w[1] {
			t.Fatalf("shortcut %d = %v, want id %q trigger %q", i, s, w[0], w[1])
		}
	}
}
