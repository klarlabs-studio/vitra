package audit_test

import (
	"testing"

	"go.klarlabs.de/vitra/audit"
)

func TestMemorySink_AppendAndList(t *testing.T) {
	s := &audit.MemorySink{}
	if err := s.Append(audit.Event{
		Kind: audit.KindCapabilityDecision, Actor: "main", Action: "fs.read", Outcome: "denied",
	}); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if len(got) != 1 || got[0].Kind != audit.KindCapabilityDecision || got[0].At.IsZero() {
		t.Fatalf("%+v", got)
	}
}
