//go:build linux && cgo && vitra_native

package linux

import (
	"os"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// The StatusNotifierItem tray against a fake StatusNotifierWatcher (which
// also plays the panel) on a private session bus: testdata/fake_sni_watcher.py.

func TestTraySNI(t *testing.T) {
	watcher := startFake(t, "fake_sni_watcher.py")
	h := newHostOnThisThread()
	got := make(chan string, 8)
	h.SetActionHandler(func(id string) { got <- id })

	tray := h.Features()[platform.FeatureTray]
	if !tray.Available || !strings.Contains(tray.Detail, "StatusNotifierItem") {
		t.Fatalf("watcher present: want StatusNotifierItem tray, got %+v", tray)
	}

	items := []platform.MenuItem{
		{ID: "app.open", Label: "Open"},
		{ID: "app.quit", Label: "Quit_now"},
	}
	if err := h.SetTray("Vitra Tray", items); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.ClearTray)
	if h.trayBackend != traySNI {
		t.Fatalf("backend %q, want sni", h.trayBackend)
	}

	reg := watcher.waitCall("RegisterStatusNotifierItem")
	if s, _ := reg["service"].(string); !strings.HasPrefix(s, "org.kde.StatusNotifierItem-") {
		t.Fatalf("registered service %q", s)
	}
	props := watcher.waitFor("item properties", func(ev map[string]any) bool { return ev["props"] != nil })["props"].(map[string]any)
	want := map[string]any{
		"Title": "Vitra Tray", "ToolTip": "Vitra Tray", "Status": "Active",
		"Category": "ApplicationStatus", "IconName": "application-x-executable",
		"Menu": "/MenuBar", "ItemIsMenu": false,
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("item %s = %v, want %v", k, props[k], v)
		}
	}
	if id, _ := props["Id"].(string); id == "" {
		t.Error("item Id is empty")
	}

	watcher.send("layout")
	layout := watcher.waitFor("layout", func(ev map[string]any) bool { return ev["layout"] != nil })
	assertTrayLayout(t, layout, []string{"Open", "Quit__now"}) // '_' escaped: labels are literal

	// A menu click reaches the action handler as the item's ID (tray.action).
	watcher.send("click Open")
	expectTrayAction(t, got, "app.open")
	watcher.send("click Quit__now")
	expectTrayAction(t, got, "app.quit")
	// Left click on the icon is tray.activate.
	watcher.send("activate")
	expectTrayAction(t, got, "tray.activate")
	// Ids the app never set are rejected and fire nothing.
	watcher.send("clickid 99")
	ev := watcher.waitFor("unknown id event", func(ev map[string]any) bool { return ev["event"] == "clicked" && ev["id"] == float64(99) })
	if ev["ok"] != false {
		t.Fatalf("Event on unknown id: %v", ev)
	}
	select {
	case id := <-got:
		t.Fatalf("unexpected action %q", id)
	case <-time.After(200 * time.Millisecond):
	}

	// Updating the tray keeps the same item and announces the new layout.
	if err := h.SetTray("Vitra Tray 2", []platform.MenuItem{{ID: "app.show", Label: "Show"}}); err != nil {
		t.Fatal(err)
	}
	watcher.waitFor("LayoutUpdated", func(ev map[string]any) bool { return ev["signal"] == "LayoutUpdated" })
	watcher.send("layout")
	layout = watcher.waitFor("layout", func(ev map[string]any) bool { return ev["layout"] != nil })
	assertTrayLayout(t, layout, []string{"Show"})
	watcher.send("props")
	props = watcher.waitFor("item properties", func(ev map[string]any) bool { return ev["props"] != nil })["props"].(map[string]any)
	if props["ToolTip"] != "Vitra Tray 2" {
		t.Fatalf("tooltip after update: %v", props["ToolTip"])
	}
	watcher.send("click Show")
	expectTrayAction(t, got, "app.show")

	// Clearing releases the item's bus name, so the watcher drops it.
	h.ClearTray()
	watcher.waitFor("item unregistered", func(ev map[string]any) bool { return ev["unregistered"] != nil })
	if h.trayBackend != trayNone {
		t.Fatalf("backend after clear %q", h.trayBackend)
	}

	// Showing it again registers a fresh item.
	if err := h.SetTray("Again", items); err != nil {
		t.Fatal(err)
	}
	watcher.waitCall("RegisterStatusNotifierItem")
}

func TestTrayFallsBackToGtkStatusIcon(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GtkStatusIcon")
	}
	requireDBusFakes(t)
	// No fake watcher: nobody owns org.kde.StatusNotifierWatcher.
	h := newHostOnThisThread()
	tray := h.Features()[platform.FeatureTray]
	if !tray.Available || !strings.Contains(tray.Detail, "GtkStatusIcon") {
		t.Fatalf("no watcher: want GtkStatusIcon tray, got %+v", tray)
	}
	if err := h.SetTray("Vitra Tray", []platform.MenuItem{{ID: "app.quit", Label: "Quit"}}); err != nil {
		t.Fatal(err)
	}
	if h.trayBackend != trayGTK {
		t.Fatalf("backend %q, want gtk", h.trayBackend)
	}
	h.ClearTray()
	if h.trayBackend != trayNone {
		t.Fatalf("backend after clear %q", h.trayBackend)
	}
}

func expectTrayAction(t *testing.T, got chan string, want string) {
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

func assertTrayLayout(t *testing.T, ev map[string]any, labels []string) {
	t.Helper()
	root, _ := ev["layout"].(map[string]any)
	if root["id"] != float64(0) || root["children-display"] != "submenu" {
		t.Fatalf("layout root %v", root)
	}
	children, _ := root["children"].([]any)
	if len(children) != len(labels) {
		t.Fatalf("layout children %v, want labels %v", children, labels)
	}
	for i, c := range children {
		item := c.(map[string]any)
		if item["label"] != labels[i] || item["id"] != float64(i+1) || item["enabled"] != true || item["visible"] != true {
			t.Fatalf("layout item %d = %v, want label %q", i, item, labels[i])
		}
	}
}
