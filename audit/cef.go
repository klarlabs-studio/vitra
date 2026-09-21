package audit

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// CEF vendor/product constants for ArcSight-compatible SIEM pipelines.
const (
	CEFVersion = 0
	CEFVendor  = "Klarlabs"
	CEFProduct = "Vitra"
	CEFDevice  = "1.0"
)

// FormatCEF renders an event as a Common Event Format line (no trailing newline).
func FormatCEF(e Event) string {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	severity := cefSeverity(e)
	name := string(e.Kind)
	if name == "" {
		name = "audit"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CEF:%d|%s|%s|%s|%s|%s|%d|",
		CEFVersion, CEFVendor, CEFProduct, CEFDevice,
		cefEscapeHeader(string(e.Kind)), cefEscapeHeader(name), severity)
	ext := []string{
		"rt=" + fmt.Sprintf("%d", e.At.UnixMilli()),
	}
	if e.Actor != "" {
		ext = append(ext, "suser="+cefEscapeExt(e.Actor))
	}
	if e.Window != "" {
		ext = append(ext, "cs1Label=window", "cs1="+cefEscapeExt(e.Window))
	}
	if e.Origin != "" {
		ext = append(ext, "cs2Label=origin", "cs2="+cefEscapeExt(e.Origin))
	}
	if e.Action != "" {
		ext = append(ext, "act="+cefEscapeExt(e.Action))
	}
	if e.Outcome != "" {
		ext = append(ext, "outcome="+cefEscapeExt(e.Outcome))
	}
	if e.Detail != "" {
		ext = append(ext, "msg="+cefEscapeExt(e.Detail))
	}
	b.WriteString(strings.Join(ext, " "))
	return b.String()
}

func cefSeverity(e Event) int {
	switch strings.ToLower(e.Outcome) {
	case "denied", "error", "failed":
		return 7
	case "allowed", "ok", "success":
		return 3
	default:
		return 5
	}
}

func cefEscapeHeader(s string) string {
	r := strings.NewReplacer("|", "\\|", "\\", "\\\\")
	return r.Replace(s)
}

func cefEscapeExt(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "=", "\\=", "\n", "\\n", "\r", "\\r")
	return r.Replace(s)
}

// CEFSink writes each event as one CEF line. List retains an in-memory copy.
type CEFSink struct {
	W io.Writer

	mu     sync.Mutex
	events []Event
}

// Append formats the event as CEF and retains it for List.
func (s *CEFSink) Append(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.Metadata != nil {
		cp := make(map[string]any, len(e.Metadata))
		for k, v := range e.Metadata {
			cp[k] = v
		}
		e.Metadata = cp
	}
	if s.W != nil {
		line := FormatCEF(e) + "\n"
		if _, err := io.WriteString(s.W, line); err != nil {
			return err
		}
	}
	s.events = append(s.events, e)
	return nil
}

// List returns a copy of retained events.
func (s *CEFSink) List() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}
