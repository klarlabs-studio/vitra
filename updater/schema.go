package updater

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ManifestSchema is the manifest format version this release writes, in the
// "schema" field. It is a string, like the IPC envelope's "protocol".
//
// Readers accept every schema written by an earlier release of the same
// major version, and refuse newer or unknown ones with ErrUnsupportedSchema
// rather than guessing at fields they do not understand.
//
// A manifest without a "schema" field was signed before versioning existed
// (0.9 and earlier) and is read as schema "1". Accepting unversioned
// manifests is deprecated: re-sign releases with this version of the CLI.
// Support is removed before 1.0.
const ManifestSchema = "1"

// ErrUnsupportedSchema reports a manifest whose "schema" this runtime does
// not know, typically one written by a newer release. Match it with
// errors.Is.
var ErrUnsupportedSchema = errors.New("unsupported update manifest schema")

// checkManifestSchema accepts the schemas this release can read. Add a case
// per format version; never drop one within a major version.
func checkManifestSchema(schema string) error {
	switch schema {
	case "": // unversioned (0.9 and earlier): read as schema 1. Deprecated.
		return nil
	case "1":
		return nil
	default:
		return fmt.Errorf("%w %q (this runtime reads up to %q)", ErrUnsupportedSchema, schema, ManifestSchema)
	}
}

// ParseManifest decodes a manifest's JSON and checks its schema. It does not
// verify the signature: call VerifyManifest or PlanInstall for that.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := checkManifestSchema(m.Schema); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
