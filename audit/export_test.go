package audit_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
)

func TestJSONLSink_AppendWritesNDJSON(t *testing.T) {
	var buf bytes.Buffer
	s := &audit.JSONLSink{W: &buf}
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	if err := s.Append(audit.Event{
		At: at, Kind: audit.KindCapabilityDecision,
		Actor: "main", Action: "fs.read", Outcome: "denied",
	}); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(buf.String())
	var got audit.Event
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("json: %v (%q)", err, line)
	}
	if got.Kind != audit.KindCapabilityDecision || got.Outcome != "denied" || !got.At.Equal(at) {
		t.Fatalf("%+v", got)
	}
	if len(s.List()) != 1 {
		t.Fatalf("list: %+v", s.List())
	}
}

func TestFormatCEF_EscapesAndSeverity(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	line := audit.FormatCEF(audit.Event{
		At: at, Kind: audit.KindCapabilityDecision,
		Actor: "a=b", Action: "fs|read", Outcome: "denied", Detail: "no\ngrant",
		Window: "main", Origin: "vitra://app",
	})
	if !strings.HasPrefix(line, "CEF:0|Klarlabs|Vitra|1.0|") {
		t.Fatalf("prefix: %q", line)
	}
	if !strings.Contains(line, "|7|") {
		t.Fatalf("denied severity: %q", line)
	}
	if !strings.Contains(line, "suser=a\\=b") {
		t.Fatalf("escape actor: %q", line)
	}
	if !strings.Contains(line, "msg=no\\ngrant") {
		t.Fatalf("escape msg: %q", line)
	}
	if strings.Contains(line, "\n") {
		t.Fatalf("newline in CEF line: %q", line)
	}
}

func TestCEFSink_AppendWritesLine(t *testing.T) {
	var buf bytes.Buffer
	s := &audit.CEFSink{W: &buf}
	if err := s.Append(audit.Event{
		Kind: audit.KindCommandInvoke, Action: "ping", Outcome: "allowed",
	}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "CEF:0|") || !strings.HasSuffix(got, "\n") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "|3|") {
		t.Fatalf("allowed severity: %q", got)
	}
}

func TestMultiSink_FanOut(t *testing.T) {
	mem := &audit.MemorySink{}
	var buf bytes.Buffer
	jsonl := &audit.JSONLSink{W: &buf}
	m := &audit.MultiSink{Sinks: []audit.Sink{mem, jsonl}}
	if err := m.Append(audit.Event{
		Kind: audit.KindPluginRegister, Actor: "demo", Outcome: "allowed",
	}); err != nil {
		t.Fatal(err)
	}
	if len(mem.List()) != 1 || len(jsonl.List()) != 1 {
		t.Fatalf("mem=%d jsonl=%d", len(mem.List()), len(jsonl.List()))
	}
	if len(m.List()) != 1 {
		t.Fatalf("multi list: %+v", m.List())
	}
	if !strings.Contains(buf.String(), `"kind":"plugin.register"`) {
		t.Fatalf("jsonl body: %q", buf.String())
	}
}
