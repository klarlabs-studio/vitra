//go:build cgo && vitra_native

package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
)

// reportPrefix is where the page sends its attack results: the navigation
// is blocked and audited (path kept, query dropped), which is how the test
// reads them back.
const reportPrefix = "https://e2e.invalid/report/"

// TestNotesE2E drives the real frontend in the native WebView (WebKitGTK,
// WKWebView, or WebView2): it saves a note and
// clicks every "Try to break it" attack, then checks the audit log and that
// the page reported every attack as refused.
func TestNotesE2E(t *testing.T) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required (run under xvfb-run)")
	}

	root, err := openVault(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	host := newHost()
	a, log, err := newApp(root, host)
	if err != nil {
		t.Fatal(err)
	}

	find := func(match func(audit.Event) bool) (audit.Event, bool) {
		for _, e := range log.List() {
			if match(e) {
				return e, true
			}
		}
		return audit.Event{}, false
	}
	waitFor := func(what string, match func(audit.Event) bool) (audit.Event, bool) {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if e, ok := find(match); ok {
				return e, true
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Errorf("timed out waiting for %s; audit log: %+v", what, log.List())
		return audit.Event{}, false
	}
	invoked := func(cmd, outcome, path string) func(audit.Event) bool {
		return func(e audit.Event) bool {
			return e.Kind == audit.KindCommandInvoke && e.Action == cmd && e.Outcome == outcome &&
				(path == "" || e.Metadata["resource_path"] == path)
		}
	}

	var report map[string]string
	go func() {
		defer a.Quit()
		welcome := root + "/Welcome.md"
		if _, ok := waitFor("Welcome.md to open", invoked("notes.read", "allowed", welcome)); !ok {
			return
		}
		js := `(function(){
var t=document.getElementById("text");t.value+="\nEdited by E2E.\n";t.dispatchEvent(new Event("input"));
document.getElementById("save").click();
document.querySelectorAll("#attack-list .attack").forEach(function(b){b.click();});
setTimeout(function(){
  var r={};document.querySelectorAll("#attack-list li").forEach(function(li){r[li.dataset.attack]=li.querySelector(".result").className+"|"+li.querySelector(".result").textContent;});
  window.location.assign("` + reportPrefix + `"+encodeURIComponent(JSON.stringify(r)));
},6000);})();`
		if err := host.Eval(mainWindow, js); err != nil {
			t.Errorf("eval: %v", err)
			return
		}
		e, ok := waitFor("the attack report", func(e audit.Event) bool {
			return e.Kind == audit.KindNavigationBlock && strings.HasPrefix(e.Detail, reportPrefix)
		})
		if !ok {
			return
		}
		raw, err := url.PathUnescape(strings.TrimPrefix(e.Detail, reportPrefix))
		if err == nil {
			err = json.Unmarshal([]byte(raw), &report)
		}
		if err != nil {
			t.Errorf("attack report %q: %v", e.Detail, err)
		}
	}()
	var runErr error
	onMainThread(func() { runErr = a.Run(t.Context()) })
	if runErr != nil {
		t.Fatal(runErr)
	}

	if b, _ := os.ReadFile(filepath.FromSlash(root + "/Welcome.md")); !strings.Contains(string(b), "Edited by E2E.") {
		t.Error("the note edit was not saved")
	}
	if _, ok := find(invoked("notes.write", "allowed", root+"/Welcome.md")); !ok {
		t.Error("no allowed notes.write in the audit log")
	}
	outside := "/etc/hosts"
	if runtime.GOOS == "windows" {
		outside = "C:/Windows/win.ini"
	}
	for code, path := range map[string]string{
		"path_out_of_scope": outside,
		"path_denied":       root + "/.private/credentials.md",
	} {
		if _, ok := find(func(e audit.Event) bool {
			return e.Outcome == "denied" && e.Metadata["code"] == code && e.Metadata["resource_path"] == path
		}); !ok {
			t.Errorf("no %s denial for %s in the audit log", code, path)
		}
	}
	if _, ok := find(func(e audit.Event) bool {
		return e.Kind == audit.KindNavigationBlock && e.Detail == "https://example.com/"
	}); !ok {
		t.Error("remote navigation was not blocked and audited")
	}
	if _, err := os.Stat(filepath.FromSlash(root + "/payload.sh")); err == nil {
		t.Error("payload.sh was written")
	}
	if len(report) != 9 {
		t.Fatalf("attack report = %v, want 9 attacks", report)
	}
	for id, res := range report {
		if !strings.HasPrefix(res, "result blocked|") {
			t.Errorf("attack %s: %s", id, res)
		}
	}
}
