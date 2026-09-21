package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

const (
	// DefaultMaxFrame is the maximum accepted JSON-line frame size (1 MiB).
	DefaultMaxFrame = 1 << 20

	FrameRequest  = "req"
	FrameResponse = "res"
	FrameMessage  = "msg"
	FrameError    = "err"
)

// Frame is one newline-delimited JSON IPC message.
type Frame struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Method  string          `json:"method,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Session is a duplex JSON-line transport between host and worker.
type Session struct {
	r         *bufio.Reader
	w         io.Writer
	closer    io.Closer
	maxFrame  int
	mu        sync.Mutex // serializes writes
	seq       atomic.Uint64
	closed    atomic.Bool
	readErr   error
	readErrMu sync.Mutex
}

// SessionOption configures a Session.
type SessionOption func(*Session)

// WithMaxFrame sets the maximum frame size in bytes.
func WithMaxFrame(n int) SessionOption {
	return func(s *Session) {
		if n > 0 {
			s.maxFrame = n
		}
	}
}

// NewSession wraps a reader/writer pair. closer is optional and closed by Close.
func NewSession(r io.Reader, w io.Writer, closer io.Closer, opts ...SessionOption) *Session {
	s := &Session{
		r:        bufio.NewReader(r),
		w:        w,
		closer:   closer,
		maxFrame: DefaultMaxFrame,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// PipePair returns two connected in-memory sessions (host and peer).
func PipePair(opts ...SessionOption) (host, peer *Session) {
	h2pR, h2pW := io.Pipe()
	p2hR, p2hW := io.Pipe()
	host = NewSession(p2hR, h2pW, multiCloser{h2pW, p2hR}, opts...)
	peer = NewSession(h2pR, p2hW, multiCloser{p2hW, h2pR}, opts...)
	return host, peer
}

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var first error
	for _, c := range m {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Close shuts down the session and underlying closer.
func (s *Session) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

// Send writes one frame. Oversized payloads and closed sessions fail closed.
func (s *Session) Send(ctx context.Context, f Frame) error {
	if s.closed.Load() {
		return errors.New("worker ipc: session closed")
	}
	if f.Type == "" {
		return errors.New("worker ipc: frame type is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if len(data)+1 > s.maxFrame {
		return fmt.Errorf("worker ipc: frame exceeds max size %d", s.maxFrame)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return errors.New("worker ipc: session closed")
	}
	if _, err := s.w.Write(append(data, '\n')); err != nil {
		s.closed.Store(true)
		return err
	}
	return nil
}

// Recv reads the next frame. Malformed or oversized lines close the session.
func (s *Session) Recv(ctx context.Context) (Frame, error) {
	if s.closed.Load() {
		return Frame{}, errors.New("worker ipc: session closed")
	}
	type result struct {
		f   Frame
		err error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := s.readLine()
		if err != nil {
			ch <- result{err: err}
			return
		}
		var f Frame
		if err := json.Unmarshal(line, &f); err != nil {
			s.failRead(fmt.Errorf("worker ipc: malformed frame: %w", err))
			_ = s.Close()
			ch <- result{err: s.readError()}
			return
		}
		if f.Type == "" {
			s.failRead(errors.New("worker ipc: missing frame type"))
			_ = s.Close()
			ch <- result{err: s.readError()}
			return
		}
		ch <- result{f: f}
	}()
	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case got := <-ch:
		return got.f, got.err
	}
}

func (s *Session) readLine() ([]byte, error) {
	var buf []byte
	for {
		part, isPrefix, err := s.r.ReadLine()
		if err != nil {
			s.failRead(err)
			_ = s.Close()
			return nil, err
		}
		buf = append(buf, part...)
		if len(buf) > s.maxFrame {
			err := fmt.Errorf("worker ipc: frame exceeds max size %d", s.maxFrame)
			s.failRead(err)
			_ = s.Close()
			return nil, err
		}
		if !isPrefix {
			return buf, nil
		}
	}
}

func (s *Session) failRead(err error) {
	s.readErrMu.Lock()
	if s.readErr == nil {
		s.readErr = err
	}
	s.readErrMu.Unlock()
	s.closed.Store(true)
}

func (s *Session) readError() error {
	s.readErrMu.Lock()
	defer s.readErrMu.Unlock()
	if s.readErr != nil {
		return s.readErr
	}
	return errors.New("worker ipc: session closed")
}

// Request sends a req frame and waits for a matching res/err by id.
func (s *Session) Request(ctx context.Context, method string, payload any) (Frame, error) {
	if method == "" {
		return Frame{}, errors.New("worker ipc: method is required")
	}
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return Frame{}, err
		}
		raw = b
	}
	id := fmt.Sprintf("%d", s.seq.Add(1))
	if err := s.Send(ctx, Frame{ID: id, Type: FrameRequest, Method: method, Payload: raw}); err != nil {
		return Frame{}, err
	}
	for {
		f, err := s.Recv(ctx)
		if err != nil {
			return Frame{}, err
		}
		switch f.Type {
		case FrameMessage:
			continue // unsolicited; caller should use Recv for those
		case FrameResponse, FrameError:
			if f.ID != id {
				continue
			}
			if f.Type == FrameError {
				if f.Error == "" {
					f.Error = "worker ipc: remote error"
				}
				return f, errors.New(f.Error)
			}
			return f, nil
		default:
			continue
		}
	}
}

// Reply sends a response frame for a prior request id.
func (s *Session) Reply(ctx context.Context, id string, payload any) error {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = b
	}
	return s.Send(ctx, Frame{ID: id, Type: FrameResponse, Payload: raw})
}

// ReplyError sends an error response for a prior request id.
func (s *Session) ReplyError(ctx context.Context, id, message string) error {
	return s.Send(ctx, Frame{ID: id, Type: FrameError, Error: message})
}
