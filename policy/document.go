package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/updater"
)

// ParseDocument decodes an MDM/fleet policy document from JSON.
func ParseDocument(r io.Reader) (Document, error) {
	var doc Document
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("policy document: %w", err)
	}
	return doc, nil
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

// Encode writes the document as indented JSON.
func (d Document) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
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
