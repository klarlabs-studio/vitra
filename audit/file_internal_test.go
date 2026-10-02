package audit

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

// A stalled disk must not stall Append: once the queue is full, Append
// refuses at once with ErrSinkFull and counts the drop. The gap is then
// recorded in the file itself, ahead of the events written after it.
func TestFileSink_FullQueueRefusesWithoutBlocking(t *testing.T) {
	s, err := NewFileSink(filepath.Join(t.TempDir(), "audit.jsonl"), 1<<20, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.fileMu.Lock() // stall the writer as a hung disk would
	var full int
	for i := 0; i < fileSinkQueue+2; i++ {
		if err := s.Append(Event{Kind: KindCommandInvoke}); errors.Is(err, ErrSinkFull) {
			full++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	s.fileMu.Unlock()
	// The writer may hold one item off the queue while it waits for the lock.
	if full < 1 || full > 2 || s.Dropped() != uint64(full) {
		t.Fatalf("full=%d dropped=%d", full, s.Dropped())
	}
	if err := s.Flush(); err != nil { // let the writer drain the queue
		t.Fatal(err)
	}
	if err := s.Append(Event{Kind: KindCommandInvoke, Action: "after"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	got := s.List()
	accepted := fileSinkQueue + 2 - full + 1
	if len(got) != accepted+1 {
		t.Fatalf("written %d lines, want %d events + 1 marker", len(got), accepted)
	}
	marker := got[0]
	if marker.Kind != KindAuditDropped || marker.Outcome != "error" {
		t.Fatalf("first line is not a drop marker: %+v", marker)
	}
	if c, _ := marker.Metadata["count"].(float64); int(c) != full {
		t.Fatalf("marker count %v, want %d", marker.Metadata["count"], full)
	}
	if want := fmt.Sprintf("%d audit events dropped: write queue full", full); marker.Detail != want {
		t.Fatalf("detail %q, want %q", marker.Detail, want)
	}
	for _, e := range got[1:] {
		if e.Kind == KindAuditDropped {
			t.Fatalf("second drop marker: %+v", e)
		}
	}
	if got[len(got)-1].Action != "after" {
		t.Fatalf("last line: %+v", got[len(got)-1])
	}
	if s.Dropped() != uint64(full) {
		t.Fatalf("Dropped() is cumulative: %d, want %d", s.Dropped(), full)
	}
}
