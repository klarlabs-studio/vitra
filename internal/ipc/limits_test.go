package ipc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/ipc"
)

func invokeEnvelope(t testing.TB, input string) []byte {
	t.Helper()
	raw, err := json.Marshal(ipc.Envelope{
		Protocol: ipc.ProtocolVersion,
		Kind:     ipc.KindInvoke,
		ID:       "1",
		Payload:  json.RawMessage(`{"command":"x","input":` + input + `}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Invokes arrive on the native message path; an oversized one must be
// rejected before any JSON is parsed.
func TestBridge_RejectsOversizedMessages(t *testing.T) {
	bridge := ipc.Bridge{Host: ipc.HostIdentity{Window: "main", Origin: domain.OriginPackagedLocal}}
	ok := invokeEnvelope(t, `"`+strings.Repeat("a", ipc.MaxMessageBytes/2)+`"`)
	if _, _, err := bridge.DecodeInvoke(ok); err != nil {
		t.Fatalf("message under the limit rejected: %v", err)
	}
	big := invokeEnvelope(t, `"`+strings.Repeat("a", ipc.MaxMessageBytes)+`"`)
	if _, _, err := bridge.DecodeInvoke(big); !errors.Is(err, ipc.ErrMessageTooLarge) {
		t.Fatalf("oversized message: err=%v", err)
	}
}

func FuzzBridge_DecodeInvoke(f *testing.F) {
	for _, seed := range []string{
		`{"protocol":"1","kind":"invoke","id":"1","payload":{"command":"a.b","input":{"x":1},"window":"admin","origin":"https://evil.example"}}`,
		`{"protocol":"1","kind":"invoke","payload":{"command":"a","input":null}}`,
		`{"protocol":"1","kind":"invoke","payload":{"command":"a","input":[[[[[]]]]]}}`,
		`{"protocol":"2","kind":"invoke","payload":{}}`,
		`{"protocol":"1","kind":"event","payload":{}}`,
		`null`, `{}`, `[`,
	} {
		f.Add([]byte(seed))
	}
	host := ipc.HostIdentity{Window: "main", Origin: domain.OriginPackagedLocal}
	bridge := ipc.Bridge{Host: host}
	f.Fuzz(func(t *testing.T, raw []byte) {
		req, _, err := bridge.DecodeInvoke(raw)
		if err != nil {
			return
		}
		// Whatever the payload claims, identity comes from the host.
		if req.Caller.Window != host.Window || req.Caller.Origin != host.Origin {
			t.Fatalf("caller %+v, want host identity %+v", req.Caller, host)
		}
		if req.Command == "" {
			t.Fatal("decoded an invoke without a command")
		}
		if len(raw) > ipc.MaxMessageBytes {
			t.Fatalf("decoded a %d-byte message over the limit", len(raw))
		}
	})
}

func TestBridge_RequiresSenderToken(t *testing.T) {
	host := ipc.HostIdentity{Window: "main", Origin: domain.OriginPackagedLocal}
	env := func(token string) []byte {
		raw, _ := json.Marshal(map[string]any{
			"protocol": ipc.ProtocolVersion, "kind": "invoke", "id": "1", "token": token,
			"payload": map[string]any{"command": "x"},
		})
		return raw
	}
	bridge := ipc.Bridge{Host: host, Token: "s3cret"}
	if _, _, err := bridge.DecodeInvoke(env("s3cret")); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, bad := range [][]byte{env(""), env("s3cre"), env("s3cret "), env("S3CRET"), []byte("{"), []byte(`{"protocol":"1","kind":"invoke","payload":{"command":"x"}}`)} {
		if _, _, err := bridge.DecodeInvoke(bad); !errors.Is(err, ipc.ErrUntrustedSender) {
			t.Fatalf("%s: err=%v, want ErrUntrustedSender", bad, err)
		}
	}
}
