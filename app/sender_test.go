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

// senderHost reports the URL of the document that sent each message, as the
// native hosts do.
type senderHost struct {
	*fakeHost
	message func(domain.WindowID, string, []byte) []byte
}

func (h *senderHost) SetMessageHandler(fn func(domain.WindowID, string, []byte) []byte) {
	h.message = fn
}

// The origin of a call is checked on every message: only documents served by
// the app's own asset server can call, whatever the window last navigated to.
func TestApp_ChecksSenderOfEveryMessage(t *testing.T) {
	sink := &audit.MemorySink{}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.sender"})
	if err != nil {
		t.Fatal(err)
	}
	rt.SetAudit(sink)
	def, _ := domain.NewCommandDefinition("demo.ping", "ping", "demo.ping")
	_ = rt.RegisterCommand(def, domain.CommandExecutorFunc(func(context.Context, domain.CommandName, any) (any, error) {
		return "pong", nil
	}))
	grant, _ := domain.NewCapabilityGrant("g", "g", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{Name: "demo.ping"}})
	_ = rt.RegisterGrant(grant)

	host := &senderHost{fakeHost: &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}}
	a, err := app.New(app.Options{
		AppID: "com.example.sender", Runtime: rt, Host: host,
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Window: app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = a.Run(context.Background()) }()
	select {
	case <-host.ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not start")
	}
	defer a.Quit()
	if host.message == nil {
		t.Fatal("app did not install a message handler on a sender-aware host")
	}

	raw, _ := json.Marshal(map[string]any{
		"protocol": "1", "kind": "invoke", "id": "1", "token": preloadToken(t, host.preload),
		"payload": map[string]any{"command": "demo.ping"},
	})
	if resp := host.message("main", a.Addr()+"/index.html", raw); resp == nil {
		t.Fatal("message from the asset server got no reply")
	} else if !json.Valid(resp) || !strings.Contains(string(resp), `"ok":true`) {
		t.Fatalf("message from the asset server: %s", resp)
	}

	for _, sender := range []string{
		"https://evil.example/",
		"about:blank",
		"",
		a.Addr() + "@evil.example/",
		"http://127.0.0.1:1/",
		"data:text/html,x",
	} {
		before := len(sink.List())
		if resp := host.message("main", sender, raw); resp != nil {
			t.Fatalf("sender %q got reply %s", sender, resp)
		}
		events := sink.List()
		if len(events) != before+1 || events[len(events)-1].Kind != audit.KindBridgeReject || events[len(events)-1].Outcome != "denied" {
			t.Fatalf("sender %q: audit = %+v", sender, events[before:])
		}
	}
	// The sender's credentials, query, and fragment stay out of the log.
	host.message("main", "https://user:pw@evil.example/x?token=s3cret#f", raw)
	if d := sink.List()[len(sink.List())-1].Detail; strings.Contains(d, "s3cret") || strings.Contains(d, "pw@") {
		t.Fatalf("audit detail leaks sender secrets: %q", d)
	}
}
