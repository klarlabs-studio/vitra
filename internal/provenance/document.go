// Package provenance generates Phase 4 release inspection metadata (SBOM hooks).
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime/debug"
	"sort"
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

// WithModulesFromBuildInfo copies the main module and dependency list from bi.
// Nil bi leaves Modules unchanged.
func (d Document) WithModulesFromBuildInfo(bi *debug.BuildInfo) Document {
	if bi == nil {
		return d
	}
	mods := make([]Module, 0, len(bi.Deps)+1)
	if bi.Main.Path != "" {
		ver := bi.Main.Version
		if ver == "" {
			ver = "(devel)"
		}
		mods = append(mods, Module{Path: bi.Main.Path, Version: ver})
	}
	for _, m := range bi.Deps {
		if m == nil || m.Path == "" {
			continue
		}
		ver := m.Version
		if ver == "" {
			ver = "(unknown)"
		}
		mods = append(mods, Module{Path: m.Path, Version: ver})
	}
	d.Modules = mods
	return d
}

// WithPluginInventory sets Plugins and the sorted Capabilities union of their perms.
func (d Document) WithPluginInventory(plugins []PluginInfo) Document {
	d.Plugins = append([]PluginInfo(nil), plugins...)
	seen := map[string]struct{}{}
	var caps []string
	for _, p := range plugins {
		for _, perm := range p.Perms {
			if perm == "" {
				continue
			}
			if _, ok := seen[perm]; ok {
				continue
			}
			seen[perm] = struct{}{}
			caps = append(caps, perm)
		}
	}
	sort.Strings(caps)
	d.Capabilities = caps
	return d
}

// JSON renders the document.
func (d Document) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
