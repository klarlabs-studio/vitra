package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// fileSinkQueue is how many encoded events a FileSink buffers before Append
// starts refusing them with ErrSinkFull.
const fileSinkQueue = 1024

// fileSinkMode is the permission for audit files: owner read/write only.
const fileSinkMode os.FileMode = 0o600

var (
	// ErrSinkClosed is returned by a FileSink after Close.
	ErrSinkClosed = errors.New("audit: sink closed")
	// ErrSinkFull is returned by FileSink.Append when the write queue is
	// full, typically because the disk is stalled. The event is dropped and
	// counted in Dropped.
	ErrSinkFull = errors.New("audit: sink queue full, event dropped")
)

// FileSink writes each event as one JSON line (the JSONLSink encoding) to a
// file and rotates it before it would grow past a maximum size. Rotated
// files are kept as path.1 (newest) … path.N (oldest); older ones are
// deleted. Files are created with mode 0600, and an existing file is
// reopened, tightened to 0600, and appended to.
//
// Append never waits for the disk: it encodes the event and queues it for a
// background writer, returning ErrSinkFull if the queue is full. Dropped
// events leave a visible gap in the log: before its next write (or at Flush
// or Close), the writer records an event of kind KindAuditDropped with
// Outcome "error" and Metadata "count" set to the number lost since the
// previous marker. Write errors are reported by Flush and Close. Call Close
// on shutdown so queued events reach the file.
type FileSink struct {
	path       string
	maxBytes   int64
	maxBackups int

	// mu guards closed and sends on queue; Close takes it exclusively.
	mu     sync.RWMutex
	closed bool
	queue  chan fileItem
	done   chan struct{}

	dropped atomic.Uint64 // cumulative, reported by Dropped
	pending atomic.Uint64 // drops not yet recorded in the file

	// fileMu guards the writer state below; held while writing or rotating.
	fileMu sync.Mutex
	file   *os.File
	size   int64
	err    error
}

// fileItem is either an encoded line or a flush marker (flushed != nil).
type fileItem struct {
	line    []byte
	flushed chan error
}

// NewFileSink opens (or creates) path for appending and starts its writer.
// maxBytes must be positive; maxBackups is the number of rotated files to
// keep and may be zero, in which case the file is truncated on rotation.
func NewFileSink(path string, maxBytes int64, maxBackups int) (*FileSink, error) {
	switch {
	case path == "":
		return nil, errors.New("audit: file sink path is empty")
	case maxBytes <= 0:
		return nil, fmt.Errorf("audit: file sink max bytes must be positive, got %d", maxBytes)
	case maxBackups < 0:
		return nil, fmt.Errorf("audit: file sink max backups must not be negative, got %d", maxBackups)
	}
	s := &FileSink{
		path:       path,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
		queue:      make(chan fileItem, fileSinkQueue),
		done:       make(chan struct{}),
	}
	if err := s.open(); err != nil {
		return nil, err
	}
	go s.run()
	return s, nil
}

// Append stamps the event, encodes it, and queues it for writing.
func (s *FileSink) Append(e Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrSinkClosed
	}
	select {
	case s.queue <- fileItem{line: b}:
		return nil
	default:
		s.dropped.Add(1)
		s.pending.Add(1)
		return ErrSinkFull
	}
}

// Flush waits until every event queued before it is written, and returns
// the first write error since the previous Flush, if any.
func (s *FileSink) Flush() error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSinkClosed
	}
	flushed := make(chan error, 1)
	s.queue <- fileItem{flushed: flushed}
	s.mu.RUnlock()
	return <-flushed
}

// Close writes the queued events, stops the writer, and closes the file. It
// returns the first unreported write error. Calling Close again is a no-op.
func (s *FileSink) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.queue)
	s.mu.Unlock()
	<-s.done

	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	err := s.takeErr()
	if s.file != nil {
		if cerr := s.file.Close(); err == nil {
			err = cerr
		}
		s.file = nil
	}
	return err
}

// Dropped reports how many events Append refused with ErrSinkFull.
func (s *FileSink) Dropped() uint64 { return s.dropped.Load() }

// List reads back the events still on disk, oldest first: the rotated files
// from path.N to path.1, then the current file. Events already rotated out
// are gone, and lines that are not audit events are skipped. List waits for
// queued events to be written; it reads files and is not meant for hot paths.
func (s *FileSink) List() []Event {
	_ = s.Flush() // a closed sink has nothing queued; write errors surface elsewhere
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	var out []Event
	for i := s.maxBackups; i >= 1; i-- {
		out = readEvents(s.backup(i), out)
	}
	return readEvents(s.path, out)
}

func (s *FileSink) run() {
	defer close(s.done)
	for it := range s.queue {
		s.fileMu.Lock()
		s.writeDropMarker()
		if it.flushed != nil {
			it.flushed <- s.takeErr()
		} else {
			s.recordErr(s.write(it.line))
		}
		s.fileMu.Unlock()
	}
	// Close stops new drops before closing the queue, so this is the last.
	s.fileMu.Lock()
	s.writeDropMarker()
	s.fileMu.Unlock()
}

// writeDropMarker records events dropped since the last marker, directly
// from the writer so the marker itself can never be dropped. If the write
// fails, the count is kept for the next attempt. Callers hold fileMu.
func (s *FileSink) writeDropMarker() {
	n := s.pending.Swap(0)
	if n == 0 {
		return
	}
	b, err := json.Marshal(Event{
		At:       time.Now().UTC(),
		Kind:     KindAuditDropped,
		Outcome:  "error",
		Detail:   fmt.Sprintf("%d audit events dropped: write queue full", n),
		Metadata: map[string]any{"count": n},
	})
	if err == nil {
		err = s.write(append(b, '\n'))
	}
	if err != nil {
		s.pending.Add(n)
		s.recordErr(err)
	}
}

// write appends one line, rotating first if it would exceed maxBytes. A line
// longer than maxBytes is written alone into a fresh file. Rotation errors
// are recorded; the returned error is whether the line was written. Callers
// hold fileMu.
func (s *FileSink) write(line []byte) error {
	if s.file != nil && s.size > 0 && s.size+int64(len(line)) > s.maxBytes {
		s.recordErr(s.rotate())
	}
	if s.file == nil {
		if err := s.open(); err != nil {
			return err
		}
	}
	n, err := s.file.Write(line)
	s.size += int64(n)
	return err
}

// open opens the current file for appending with mode 0600. Callers hold
// fileMu or own s exclusively.
func (s *FileSink) open() error {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, fileSinkMode)
	if err != nil {
		return fmt.Errorf("audit: open %s: %w", s.path, err)
	}
	fi, err := f.Stat()
	if err == nil && !fi.Mode().IsRegular() {
		err = fmt.Errorf("audit: %s is not a regular file", s.path)
	}
	if err == nil && fi.Mode().Perm() != fileSinkMode {
		err = f.Chmod(fileSinkMode)
	}
	if err != nil {
		_ = f.Close()
		return err
	}
	s.file, s.size = f, fi.Size()
	return nil
}

// rotate closes the current file and shifts path → path.1 → … → path.N,
// deleting the oldest. With no backups the current file is removed. The next
// write opens a fresh file. Callers hold fileMu.
func (s *FileSink) rotate() error {
	err := s.file.Close()
	s.file, s.size = nil, 0
	if s.maxBackups == 0 {
		return firstErr(err, removeIfExists(s.path))
	}
	err = firstErr(err, removeIfExists(s.backup(s.maxBackups)))
	for i := s.maxBackups - 1; i >= 1; i-- {
		err = firstErr(err, renameIfExists(s.backup(i), s.backup(i+1)))
	}
	return firstErr(err, renameIfExists(s.path, s.backup(1)))
}

func (s *FileSink) backup(i int) string { return s.path + "." + strconv.Itoa(i) }

func (s *FileSink) recordErr(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func (s *FileSink) takeErr() error {
	err := s.err
	s.err = nil
	return err
}

func firstErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func renameIfExists(from, to string) error {
	if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// readEvents appends the audit events in path to out, skipping lines that
// do not decode. A missing file contributes nothing.
func readEvents(path string, out []Event) []Event {
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Kind != "" {
			out = append(out, e)
		}
	}
	return out
}
