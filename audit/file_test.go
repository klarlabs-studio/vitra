package audit_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/vitra/audit"
)

// lineEvent returns an event whose JSON line has a fixed length for a given i
// width, so rotation thresholds are predictable.
func lineEvent(i int) audit.Event {
	return audit.Event{
		At:     time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC),
		Kind:   audit.KindCommandInvoke,
		Action: fmt.Sprintf("cmd-%04d", i),
	}
}

func lineLen(t *testing.T, e audit.Event) int64 {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return int64(len(b) + 1)
}

func readLines(t *testing.T, path string) []audit.Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []audit.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e audit.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("%s: bad line %q: %v", path, sc.Text(), err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func newFileSink(t *testing.T, path string, maxBytes int64, maxBackups int) *audit.FileSink {
	t.Helper()
	s, err := audit.NewFileSink(path, maxBytes, maxBackups)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func appendAll(t *testing.T, s *audit.FileSink, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		if err := s.Append(lineEvent(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

func TestNewFileSink_RejectsBadArguments(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name       string
		path       string
		maxBytes   int64
		maxBackups int
	}{
		{"empty path", "", 1024, 1},
		{"zero max bytes", filepath.Join(dir, "a.jsonl"), 0, 1},
		{"negative max bytes", filepath.Join(dir, "a.jsonl"), -1, 1},
		{"negative backups", filepath.Join(dir, "a.jsonl"), 1024, -1},
		{"directory path", dir, 1024, 1},
		{"missing parent", filepath.Join(dir, "nope", "a.jsonl"), 1024, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := audit.NewFileSink(tc.path, tc.maxBytes, tc.maxBackups)
			if err == nil {
				_ = s.Close()
				t.Fatal("want error")
			}
		})
	}
}

func TestFileSink_WritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	s := newFileSink(t, path, 1<<20, 3)
	if err := s.Append(audit.Event{Kind: audit.KindCapabilityDecision, Action: "fs.read", Outcome: "denied"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	got := readLines(t, path)
	if len(got) != 1 || got[0].Action != "fs.read" || got[0].At.IsZero() {
		t.Fatalf("%+v", got)
	}
}

func TestFileSink_RotatesBeforeExceedingMaxBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	n := lineLen(t, lineEvent(0))
	s := newFileSink(t, path, 3*n, 5)
	appendAll(t, s, 0, 7)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	// 7 lines of 3 per file: current has 1, .1 has 3 (newest), .2 has 3.
	for _, p := range []string{path, path + ".1", path + ".2"} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() > 3*n {
			t.Fatalf("%s is %d bytes, max %d", p, fi.Size(), 3*n)
		}
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Action != "cmd-0006" {
		t.Fatalf("current: %+v", got)
	}
	if got := readLines(t, path+".1"); len(got) != 3 || got[0].Action != "cmd-0003" {
		t.Fatalf(".1: %+v", got)
	}
	if got := readLines(t, path+".2"); len(got) != 3 || got[0].Action != "cmd-0000" {
		t.Fatalf(".2: %+v", got)
	}
}

func TestFileSink_KeepsOnlyMaxBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	n := lineLen(t, lineEvent(0))
	s := newFileSink(t, path, n, 2) // one line per file
	appendAll(t, s, 0, 10)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Action != "cmd-0009" {
		t.Fatalf("current: %+v", got)
	}
	if got := readLines(t, path+".1"); len(got) != 1 || got[0].Action != "cmd-0008" {
		t.Fatalf(".1: %+v", got)
	}
	if got := readLines(t, path+".2"); len(got) != 1 || got[0].Action != "cmd-0007" {
		t.Fatalf(".2: %+v", got)
	}
	if _, err := os.Stat(path + ".3"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".3 should not exist: %v", err)
	}
	list := s.List()
	if len(list) != 3 || list[0].Action != "cmd-0007" || list[2].Action != "cmd-0009" {
		t.Fatalf("list: %+v", list)
	}
}

func TestFileSink_ZeroBackupsTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	n := lineLen(t, lineEvent(0))
	s := newFileSink(t, path, 2*n, 0)
	appendAll(t, s, 0, 5)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Action != "cmd-0004" {
		t.Fatalf("current: %+v", got)
	}
	if _, err := os.Stat(path + ".1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".1 should not exist: %v", err)
	}
}

func TestFileSink_ReopensAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	n := lineLen(t, lineEvent(0))

	first, err := audit.NewFileSink(path, 3*n, 2)
	if err != nil {
		t.Fatal(err)
	}
	appendAll(t, first, 0, 2)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := newFileSink(t, path, 3*n, 2)
	appendAll(t, second, 2, 4)
	if err := second.Flush(); err != nil {
		t.Fatal(err)
	}
	// The existing two lines count toward the limit: the 4th line rotates.
	if got := readLines(t, path+".1"); len(got) != 3 || got[0].Action != "cmd-0000" {
		t.Fatalf(".1: %+v", got)
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Action != "cmd-0003" {
		t.Fatalf("current: %+v", got)
	}
	if got := second.List(); len(got) != 4 {
		t.Fatalf("list: %+v", got)
	}
}

func TestFileSink_FilesAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	// A pre-existing world-readable file is tightened on open.
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	n := lineLen(t, lineEvent(0))
	s := newFileSink(t, path, n, 1)
	appendAll(t, s, 0, 2)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + ".1"} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s mode %o, want 600", p, perm)
		}
	}
}

func TestFileSink_ConcurrentAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	n := lineLen(t, lineEvent(0))
	const workers, each = 8, 200
	s, err := audit.NewFileSink(path, 50*n, 40) // room for every line
	if err != nil {
		t.Fatal(err)
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted int
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				err := s.Append(lineEvent(w*each + i))
				switch {
				case err == nil:
					mu.Lock()
					accepted++
					mu.Unlock()
				case errors.Is(err, audit.ErrSinkFull):
				default:
					t.Errorf("append: %v", err)
				}
			}
		}(w)
	}
	wg.Wait()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if len(got) != accepted {
		t.Fatalf("wrote %d events, accepted %d", len(got), accepted)
	}
	if uint64(workers*each-accepted) != s.Dropped() {
		t.Fatalf("dropped %d, want %d", s.Dropped(), workers*each-accepted)
	}
	seen := make(map[string]bool, len(got))
	for _, e := range got {
		if seen[e.Action] {
			t.Fatalf("duplicate %s", e.Action)
		}
		seen[e.Action] = true
	}
}

func TestFileSink_AppendAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	s, err := audit.NewFileSink(path, 1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if err := s.Append(lineEvent(0)); !errors.Is(err, audit.ErrSinkClosed) {
		t.Fatalf("append: %v", err)
	}
	if err := s.Flush(); !errors.Is(err, audit.ErrSinkClosed) {
		t.Fatalf("flush: %v", err)
	}
}

func TestFileSink_ListSkipsForeignLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newFileSink(t, path, 1<<20, 1)
	appendAll(t, s, 0, 1)
	got := s.List()
	if len(got) != 1 || !strings.HasPrefix(got[0].Action, "cmd-") {
		t.Fatalf("%+v", got)
	}
}
