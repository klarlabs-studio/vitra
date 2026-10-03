package audit_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
)

func TestMarshalEvent_WritesSchema(t *testing.T) {
	if audit.EventSchema != "1" {
		t.Fatalf("EventSchema = %q", audit.EventSchema)
	}
	at := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	b, err := audit.MarshalEvent(audit.Event{At: at, Kind: audit.KindCommandInvoke, Action: "notes.save"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"1","at":"2026-10-03T12:00:00Z","kind":"command.invoke","action":"notes.save"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
	got, err := audit.ParseEvent(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != audit.KindCommandInvoke || got.Action != "notes.save" || !got.At.Equal(at) {
		t.Fatalf("%+v", got)
	}
}

func TestParseEvent_RejectsUnknownSchema(t *testing.T) {
	_, err := audit.ParseEvent([]byte(`{"schema":"2","at":"2026-10-03T12:00:00Z","kind":"command.invoke"}`))
	if !errors.Is(err, audit.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
	if _, err := audit.ParseEvent([]byte(`{"schema":1,"kind":"command.invoke"}`)); err == nil {
		t.Fatal("numeric schema must be rejected")
	}
}

func TestParseEvent_LegacyWithoutSchemaAcceptedAsV1(t *testing.T) {
	e, err := audit.ParseEvent([]byte(`{"at":"2026-10-03T12:00:00Z","kind":"command.invoke","action":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != audit.KindCommandInvoke || e.Action != "x" {
		t.Fatalf("%+v", e)
	}
}

func TestJSONLSink_WritesSchema(t *testing.T) {
	var buf bytes.Buffer
	s := &audit.JSONLSink{W: &buf}
	if err := s.Append(audit.Event{Kind: audit.KindCommandInvoke}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), `{"schema":"1",`) {
		t.Fatalf("line does not lead with schema: %s", buf.String())
	}
}

func TestFileSink_WritesSchemaAndSkipsUnknownOnList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	// A file holding a legacy line and a line from a newer format.
	seed := `{"at":"2026-10-03T12:00:00Z","kind":"command.invoke","action":"legacy"}` + "\n" +
		`{"schema":"2","at":"2026-10-03T12:00:01Z","kind":"command.invoke","action":"future"}` + "\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newFileSink(t, path, 1<<20, 1)
	if err := s.Append(audit.Event{Kind: audit.KindCommandInvoke, Action: "current"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if !strings.HasPrefix(lines[len(lines)-1], `{"schema":"1",`) {
		t.Fatalf("written line has no schema: %s", lines[len(lines)-1])
	}
	var actions []string
	for _, e := range s.List() {
		actions = append(actions, e.Action)
	}
	if strings.Join(actions, ",") != "legacy,current" {
		t.Fatalf("List = %v, want legacy and current only", actions)
	}
}

func FuzzParseEvent(f *testing.F) {
	f.Add([]byte(`{"schema":"1","at":"2026-10-03T12:00:00Z","kind":"command.invoke"}`))
	f.Add([]byte(`{"kind":"x"}`))
	f.Add([]byte(`{"schema":"2"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := audit.ParseEvent(data)
		if err != nil {
			return
		}
		b, err := audit.MarshalEvent(e)
		if err != nil {
			return // e.g. a metadata value JSON cannot re-encode
		}
		if _, err := audit.ParseEvent(b); err != nil {
			t.Fatalf("re-encoded event does not parse: %v\n%s", err, b)
		}
	})
}
