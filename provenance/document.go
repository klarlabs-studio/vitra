// Package provenance generates Phase 4 release inspection metadata (SBOM hooks).
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Document is an inspectable release provenance record.
type Document struct {
	AppID          string       `json:"app_id"`
	Version        string       `json:"version"`
	BuiltAt        time.Time    `json:"built_at"`
	GoVersion      string       `json:"go_version"`
	Modules        []Module     `json:"modules"`
	Plugins        []PluginInfo `json:"plugins"`
	Capabilities   []string     `json:"capabilities"`
	ArtifactSHA256 string       `json:"artifact_sha256,omitempty"`
}

// Module is a Go module dependency entry.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// PluginInfo is a plugin inventory entry.
type PluginInfo struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	Perms   []string `json:"permissions"`
}

// NewDocument constructs a provenance document.
func NewDocument(appID, version, goVersion string) Document {
	return Document{
		AppID:     appID,
		Version:   version,
		BuiltAt:   time.Now().UTC(),
		GoVersion: goVersion,
	}
}

// WithArtifactDigest sets the artifact hash.
func (d Document) WithArtifactDigest(artifact []byte) Document {
	sum := sha256.Sum256(artifact)
	d.ArtifactSHA256 = hex.EncodeToString(sum[:])
	return d
}

// JSON renders the document.
func (d Document) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
