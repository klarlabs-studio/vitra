package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
)

// rejectHost is a host that drops some bridge messages natively (as macOS
// does for subframes) and reports them.
type rejectHost struct {
	*fakeHost
	reject func(domain.WindowID, string)
}

func (h *rejectHost) SetRejectHandler(fn func(domain.WindowID, string)) { h.reject = fn }

// Blocked navigations and dropped bridge messages are attacks or bugs an
// operator wants to see, so they are audited like denied invokes.
func TestApp_AuditsRejectedMessagesAndBlockedNavigation(t *testing.T) {
	sink := &audit.MemorySink{}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.audit"})
	if err != nil {
		t.Fatal(err)
	}
	rt.SetAudit(sink)
	host := &rejectHost{fakeHost: &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}}
	application, err := app.New(app.Options{
		AppID:   "com.example.audit",
		Assets:  fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = application.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	defer application.Quit()

	last := func() audit.Event {
		t.Helper()
		events := sink.List()
		if len(events) == 0 {
			t.Fatal("no audit event recorded")
		}
		return events[len(events)-1]
	}

	// Allowed navigation is routine and not audited.
	if !host.nav("main", application.Addr()+"/") {
		t.Fatal("local navigation must be allowed")
	}
	if n := len(sink.List()); n != 0 {
		t.Fatalf("allowed navigation audited: %+v", sink.List())
	}

	// Blocked navigation is audited without query, fragment, or credentials,
	// which can carry secrets.
	if host.nav("main", "https://user:hunter2@evil.example/steal?token=s3cret#frag") {
		t.Fatal("external navigation must be blocked")
	}
	e := last()
	if e.Kind != audit.KindNavigationBlock || e.Window != "main" || e.Outcome != "denied" {
		t.Fatalf("navigation audit = %+v", e)
	}
	if e.Detail != "https://evil.example/steal" {
		t.Fatalf("navigation detail = %q, want redacted URL", e.Detail)
	}

	// A message without the window's sender token is dropped and audited.
	forged, _ := json.Marshal(map[string]any{
		"protocol": "1", "kind": "invoke", "id": "1", "token": strings.Repeat("a", 64),
		"payload": map[string]any{"command": "fs.read"},
	})
	if resp := host.invoke("main", domain.OriginPackagedLocal, forged); resp != nil {
		t.Fatalf("forged message got reply %s", resp)
	}
	e = last()
	if e.Kind != audit.KindBridgeReject || e.Window != "main" || e.Origin != string(domain.OriginPackagedLocal) || e.Outcome != "denied" {
		t.Fatalf("forged message audit = %+v", e)
	}

	// Unparseable messages cannot prove their sender: dropped and audited.
	if resp := host.invoke("main", domain.OriginPackagedLocal, []byte("{not json")); resp != nil {
		t.Fatalf("unparseable message got reply %s", resp)
	}
	if e = last(); e.Kind != audit.KindBridgeReject || e.Outcome != "denied" {
		t.Fatalf("unparseable message audit = %+v", e)
	}

	// A message from the top frame that is otherwise invalid gets a
	// bad_request reply and is audited as an error.
	invalid, _ := json.Marshal(map[string]any{
		"protocol": "9", "kind": "invoke", "id": "2", "token": preloadToken(t, host.preload),
		"payload": map[string]any{"command": "fs.read"},
	})
	if resp := host.invoke("main", domain.OriginPackagedLocal, invalid); resp == nil {
		t.Fatal("invalid message from the top frame must get a bad_request reply")
	}
	if e = last(); e.Kind != audit.KindBridgeReject || e.Outcome != "error" || !strings.Contains(e.Detail, "protocol") {
		t.Fatalf("invalid message audit = %+v", e)
	}

	// Hosts that drop messages natively report them too.
	if host.reject == nil {
		t.Fatal("app did not install a reject handler")
	}
	host.reject("main", "message from a subframe")
	e = last()
	if e.Kind != audit.KindBridgeReject || e.Window != "main" || e.Outcome != "denied" || e.Detail != "message from a subframe" {
		t.Fatalf("native reject audit = %+v", e)
	}
}
