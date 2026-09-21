package darwin

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTrySingleInstance_Exclusive(t *testing.T) {
	h := New()
	held, release, err := h.TrySingleInstance("vitra-darwin-si-test")
	if err != nil || !held {
		t.Fatalf("first lock: held=%v err=%v", held, err)
	}
	defer release()

	h2 := New()
	held2, release2, err := h2.TrySingleInstance("vitra-darwin-si-test")
	if err != nil {
		t.Fatalf("second lock err: %v", err)
	}
	if held2 {
		release2()
		t.Fatal("second lock should not be held")
	}
}

func TestDeepLinksFromArgs(t *testing.T) {
	got := DeepLinksFromArgs([]string{"app", "vitra://open/x", "--flag", "https://example.com/a", "not-a-url"})
	if len(got) != 2 || got[0] != "vitra://open/x" || got[1] != "https://example.com/a" {
		t.Fatalf("got %#v", got)
	}
}

func TestDeepLinkBridge_Handoff(t *testing.T) {
	const appID = "vitra-darwin-dl-test"
	h := New()
	held, release, err := h.TrySingleInstance(appID)
	if err != nil || !held {
		t.Fatalf("lock: held=%v err=%v", held, err)
	}
	defer release()

	var got atomic.Value
	stop, err := h.StartDeepLinkBridge(appID, func(raw string) { got.Store(raw) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	h2 := New()
	ok, err := h2.ForwardToPrimary(appID, []string{"vitra://open/project"})
	if err != nil || !ok {
		t.Fatalf("forward: ok=%v err=%v", ok, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if v, _ := got.Load().(string); v == "vitra://open/project" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("did not receive deep link, got=%v", got.Load())
}

func TestForwardToPrimary_NoListener(t *testing.T) {
	h := New()
	ok, err := h.ForwardToPrimary("vitra-darwin-dl-missing", []string{"vitra://x"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected ok=false when no primary")
	}
}
