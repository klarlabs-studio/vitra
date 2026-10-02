//go:build linux && cgo && vitra_native

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/platform/linux"
)

// tourDone is where the tour navigates when it has finished; the blocked
// navigation shows up in the audit log.
const tourDone = "https://tour.invalid/done"

// tourJS walks through the app at a pace a viewer can follow: edit and save
// a note, then run each attack one by one, then show only the refusals.
const tourJS = `(function(){
const wait=(ms)=>new Promise((r)=>setTimeout(r,ms));
function flash(el){el.style.outline="3px solid #2f5bd3";el.style.outlineOffset="2px";setTimeout(()=>{el.style.outline="";},900);}
async function click(el,pause){(el.closest("li")||el).scrollIntoView({block:"nearest"});flash(el);await wait(350);el.click();await wait(pause);(el.closest("li")||el).scrollIntoView({block:"nearest"});}
(async function(){
  await wait(2500);
  await click(document.querySelector('.tree-note[data-path$="/Shortcuts.md"]'),1200);
  await click(document.getElementById("tab-edit"),700);
  const t=document.getElementById("text");
  t.focus();t.selectionStart=t.selectionEnd=t.value.length;
  for(const ch of "- **Esc**: close the new-note box\n"){t.value+=ch;t.dispatchEvent(new Event("input"));await wait(45);}
  await wait(500);
  await click(document.getElementById("save"),1800);
  for(const b of document.querySelectorAll("#attack-list .attack")){await click(b,1900);}
  await click(document.getElementById("filter-refused"),4000);
  window.location.assign("` + tourDone + `");
})();})();`

// TestNotesTour plays the tour in a real window for recording the README
// demo (scripts/record-notes-demo.sh). It only runs when asked to.
func TestNotesTour(t *testing.T) {
	if os.Getenv("VITRA_RECORD_TOUR") != "1" {
		t.Skip("set VITRA_RECORD_TOUR=1 to play the demo tour")
	}
	// VITRA_TOUR_VAULT gives the recording a readable path in the footer.
	dir := os.Getenv("VITRA_TOUR_VAULT")
	if dir == "" {
		dir = filepath.Join(t.TempDir(), "VitraNotes")
	}
	root, err := openVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	host := linux.New()
	a, log, err := newApp(root, host)
	if err != nil {
		t.Fatal(err)
	}
	seen := func(match func(audit.Event) bool) bool {
		for _, e := range log.List() {
			if match(e) {
				return true
			}
		}
		return false
	}
	waitFor := func(match func(audit.Event) bool, d time.Duration) bool {
		for deadline := time.Now().Add(d); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
			if seen(match) {
				return true
			}
		}
		return false
	}
	go func() {
		defer a.Quit()
		opened := waitFor(func(e audit.Event) bool {
			return e.Action == "notes.read" && e.Outcome == "allowed"
		}, 30*time.Second)
		if !opened {
			t.Error("the welcome note never opened")
			return
		}
		if err := host.Eval(mainWindow, tourJS); err != nil {
			t.Errorf("eval: %v", err)
			return
		}
		if !waitFor(func(e audit.Event) bool {
			return e.Kind == audit.KindNavigationBlock && strings.HasPrefix(e.Detail, tourDone)
		}, 90*time.Second) {
			t.Error("the tour did not finish")
		}
	}()
	var runErr error
	onMainThread(func() { runErr = a.Run(t.Context()) })
	if runErr != nil {
		t.Fatal(runErr)
	}
}
