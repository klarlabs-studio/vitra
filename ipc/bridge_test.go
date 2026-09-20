package ipc_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/ipc"
)

func TestBridge_DiscardsSpoofedCallerIdentity(t *testing.T) {
	// Security invariants 3 & 4: frontend-claimed identity is never trusted;
	// host identity wins.
	bridge := ipc.Bridge{Host: ipc.HostIdentity{
		Window: "main",
		Origin: domain.OriginPackagedLocal,
	}}

	raw, err := json.Marshal(ipc.Envelope{
		Protocol: ipc.ProtocolVersion,
		Kind:     ipc.KindInvoke,
		ID:       "1",
		Payload: mustJSON(t, ipc.InvokePayload{
			Command:       "project.open",
			ResourcePath:  "/project/a",
			ClaimedWindow: "admin",
			ClaimedOrigin: "https://evil.example",
			Input:         mustJSON(t, map[string]any{"path": "/project/a"}),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	req, id, err := bridge.DecodeInvoke(raw)
	if err != nil {
		t.Fatal(err)
	}
	if id != "1" {
		t.Fatalf("id=%q", id)
	}
	if req.Caller.Window != "main" || req.Caller.Origin != domain.OriginPackagedLocal {
		t.Fatalf("host identity discarded: %+v", req.Caller)
	}
	if req.Command != "project.open" {
		t.Fatalf("command=%q", req.Command)
	}
}

func TestBridge_RejectsWrongProtocol(t *testing.T) {
	bridge := ipc.Bridge{Host: ipc.HostIdentity{Window: "main", Origin: domain.OriginPackagedLocal}}
	raw, _ := json.Marshal(ipc.Envelope{
		Protocol: "999",
		Kind:     ipc.KindInvoke,
		Payload:  mustJSON(t, ipc.InvokePayload{Command: "x"}),
	})
	_, _, err := bridge.DecodeInvoke(raw)
	if err == nil || !strings.Contains(err.Error(), "unsupported ipc protocol") {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestEncodeResultAndError(t *testing.T) {
	out, err := ipc.EncodeResult("1", "project.open", map[string]string{"ok": "true"})
	if err != nil {
		t.Fatal(err)
	}
	var env ipc.Envelope
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatal(err)
	}
	if env.Kind != ipc.KindInvokeResult || env.Protocol != ipc.ProtocolVersion {
		t.Fatalf("%+v", env)
	}

	errBytes, err := ipc.EncodeError("1", string(domain.DenialNoGrant), "denied")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(errBytes, &env); err != nil {
		t.Fatal(err)
	}
	if env.Kind != ipc.KindError {
		t.Fatalf("kind=%s", env.Kind)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
