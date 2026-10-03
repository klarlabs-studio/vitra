//go:build cgo && vitra_native

package main

import (
	"os"
	"runtime"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/platform"
)

// TestMenubarE2E runs the menu bar app in the native WebView (WebKitGTK,
// WKWebView, or WebView2): the hidden panel follows the usage, its
// clipboard read is refused, the tray shows and hides the panel, and the
// panel closes itself.
func TestMenubarE2E(t *testing.T) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required (run under xvfb-run)")
	}
	sink := &audit.MemorySink{}
	host := newHost()
	m, err := newMenubar(host, t.TempDir(), sink)
	if err != nil {
		t.Fatal(err)
	}
	panels := host.(platform.Panels)
	waitFor := func(what string, match func(audit.Event) bool) bool {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
			for _, e := range sink.List() {
				if match(e) {
					return true
				}
			}
		}
		t.Errorf("timed out waiting for %s; audit log: %+v", what, sink.List())
		return false
	}
	invoked := func(cmd, outcome string) func(audit.Event) bool {
		return func(e audit.Event) bool {
			return e.Kind == audit.KindCommandInvoke && e.Action == cmd && e.Outcome == outcome && e.Window == string(panelWindow)
		}
	}

	go func() {
		defer m.app.Quit()
		// The panel loads hidden and follows the usage at once.
		if !waitFor("the panel to follow the usage", invoked("usage.follow", "allowed")) {
			return
		}
		if !waitFor("the clipboard read to be refused", invoked("clipboard.read", "denied")) {
			return
		}
		if err := m.app.ShowTrayPanel(); err != nil {
			t.Errorf("ShowTrayPanel: %v", err)
			return
		}
		if shown, err := panels.PanelShown(panelWindow); err != nil || !shown {
			t.Errorf("panel after ShowTrayPanel: shown=%v err=%v", shown, err)
			return
		}
		// The panel closes itself through its own command.
		js := `window.vitra.invoke("panel.close", {}, "")`
		if err := host.(platform.ScriptEvaluator).Eval(panelWindow, js); err != nil {
			t.Errorf("eval: %v", err)
			return
		}
		if !waitFor("the panel to close itself", invoked("panel.close", "allowed")) {
			return
		}
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if shown, _ := panels.PanelShown(panelWindow); !shown {
				return
			}
		}
		t.Error("panel still shown after panel.close")
	}()
	var runErr error
	onMainThread(func() { runErr = m.Run(t.Context()) })
	if runErr != nil {
		t.Fatal(runErr)
	}
}
