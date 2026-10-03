package audit

import (
	"encoding/json"
	"errors"
	"fmt"
)

// EventSchema is the audit event JSON format version this release writes, in
// the "schema" field of every line JSONLSink and FileSink emit. It is a
// string, like the IPC envelope's "protocol".
//
// ParseEvent accepts every schema written by an earlier release of the same
// major version, and refuses newer or unknown ones with ErrUnsupportedSchema.
// A line without a "schema" field (0.9 and earlier) is read as schema "1".
//
// CEF output is not affected: its format is versioned by the CEF header.
const EventSchema = "1"

// ErrUnsupportedSchema reports an audit event whose "schema" this runtime
// does not know, typically one written by a newer release. Match it with
// errors.Is.
var ErrUnsupportedSchema = errors.New("unsupported audit event schema")

// eventV1 is the JSON form of schema 1 (and of unversioned lines).
type eventV1 struct {
	Schema string `json:"schema,omitempty"`
	Event
}

// MarshalEvent encodes e as one line of audit JSON (without the newline),
// versioned with EventSchema. Custom sinks that write JSON should use it so
// their output matches JSONLSink and FileSink.
func MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(eventV1{Schema: EventSchema, Event: e})
}

// ParseEvent decodes one line of audit JSON. A schema this runtime does not
// know fails with ErrUnsupportedSchema.
func ParseEvent(data []byte) (Event, error) {
	var head struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Event{}, fmt.Errorf("audit event: %w", err)
	}
	switch head.Schema {
	case "", "1": // "" is unversioned (0.9 and earlier), read as 1.
		var v eventV1
		if err := json.Unmarshal(data, &v); err != nil {
			return Event{}, fmt.Errorf("audit event: %w", err)
		}
		return v.Event, nil
	default:
		return Event{}, fmt.Errorf("audit event: %w %q (this runtime reads up to %q)",
			ErrUnsupportedSchema, head.Schema, EventSchema)
	}
}
