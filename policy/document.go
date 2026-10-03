package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/updater"
)

// DocumentSchema is the policy document format version this release writes,
// in the "schema" field. It is a string, like the IPC envelope's "protocol".
//
// ParseDocument accepts every schema written by an earlier release of the
// same major version, and refuses newer or unknown ones with
// ErrUnsupportedSchema, so a runtime never half-applies a policy it does not
// fully understand.
//
// A document without a "schema" field (0.9 and earlier) is read as schema
// "1". Accepting unversioned documents is deprecated: add "schema": "1", or
// re-save with Document.Save. Support is removed before 1.0.
const DocumentSchema = "1"

// ErrUnsupportedSchema reports a policy document whose "schema" this runtime
// does not know, typically one written for a newer release. Match it with
// errors.Is.
var ErrUnsupportedSchema = errors.New("unsupported policy document schema")

// documentV1 is the JSON form of schema 1 (and of unversioned documents).
type documentV1 struct {
	Schema string `json:"schema,omitempty"`
	Document
}

// ParseDocument decodes an MDM/fleet policy document from JSON. Unknown
// fields are rejected, and so is a schema this runtime does not know
// (ErrUnsupportedSchema).
func ParseDocument(r io.Reader) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Document{}, fmt.Errorf("policy document: %w", err)
	}
	// Read the version first: a newer schema may carry fields this release
	// would otherwise report as unknown.
	var head struct {
		Schema string `json:"schema"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&head); err != nil {
		return Document{}, fmt.Errorf("policy document: %w", err)
	}
	switch head.Schema {
	case "", "1": // "" is unversioned (0.9 and earlier), read as 1. Deprecated.
		var v documentV1
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return Document{}, fmt.Errorf("policy document: %w", err)
		}
		return v.Document, nil
	default:
		return Document{}, fmt.Errorf("policy document: %w %q (this runtime reads up to %q)",
			ErrUnsupportedSchema, head.Schema, DocumentSchema)
	}
}

// LoadDocument reads a JSON policy document from path.
func LoadDocument(path string) (Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer func() { _ = f.Close() }()
	return ParseDocument(f)
}

// Encode writes the document as indented JSON, versioned with
// DocumentSchema.
func (d Document) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(documentV1{Schema: DocumentSchema, Document: d}); err != nil {
		return fmt.Errorf("policy document: %w", err)
	}
	return nil
}

// Save writes the document as indented JSON to path (0644).
func (d Document) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return d.Encode(f)
}

// Document returns a copy of the engine's effective policy document.
func (e *Engine) Document() Document {
	if e == nil {
		return Document{}
	}
	d := e.doc
	if d.DenyPermissions != nil {
		d.DenyPermissions = append([]domain.PermissionName(nil), d.DenyPermissions...)
	}
	if d.AllowedUpdateChannels != nil {
		d.AllowedUpdateChannels = append([]updater.Channel(nil), d.AllowedUpdateChannels...)
	}
	return d
}
