package audit

import (
	"errors"
	"path/filepath"
	"testing"
)

// A stalled disk must not stall Append: once the queue is full, Append
// refuses at once with ErrSinkFull and counts the drop.
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
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got := len(s.List()); got != fileSinkQueue+2-full {
		t.Fatalf("written %d, want %d", got, fileSinkQueue+2-full)
	}
}
