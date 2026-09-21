package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChannelStage is the result of writing a conventional update channel tree
// suitable for upload to any static HTTP host (Vitra does not host a CDN).
type ChannelStage struct {
	Root         string // outDir/{app}/{channel}
	ManifestPath string
	ArtifactPath string
}

// StageChannel writes:
//
//	{outDir}/{AppID}/{Channel}/manifest.json
//	{outDir}/{AppID}/{Channel}/{m.Artifact}
//
// The artifact basename must match m.Artifact (or be a relative path under the
// channel prefix). Callers must pass an already-signed manifest (invariant 9).
func StageChannel(outDir string, m Manifest, artifact []byte) (ChannelStage, error) {
	if strings.TrimSpace(outDir) == "" {
		return ChannelStage{}, errors.New("output directory is required")
	}
	if m.Signature == "" {
		return ChannelStage{}, errors.New("manifest signature is required")
	}
	if m.AppID == "" || m.Channel == "" || m.Artifact == "" || m.SHA256 == "" {
		return ChannelStage{}, errors.New("manifest app_id, channel, artifact, and sha256 are required")
	}
	if err := VerifyArtifactDigest(m, artifact); err != nil {
		return ChannelStage{}, err
	}
	src := ChannelSource{BaseURL: "https://example.invalid/", AppID: m.AppID, Channel: m.Channel}
	if _, _, err := src.validatedIdentity(); err != nil {
		return ChannelStage{}, err
	}
	if strings.Contains(m.Artifact, "://") {
		return ChannelStage{}, errors.New("stage requires a relative artifact name, not an absolute URL")
	}
	if err := validateRelativeArtifact(m.Artifact); err != nil {
		return ChannelStage{}, err
	}

	root := filepath.Join(outDir, m.AppID, string(m.Channel))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return ChannelStage{}, err
	}
	artPath := filepath.Join(root, filepath.FromSlash(m.Artifact))
	if err := os.MkdirAll(filepath.Dir(artPath), 0o755); err != nil {
		return ChannelStage{}, err
	}
	if err := os.WriteFile(artPath, artifact, 0o644); err != nil {
		return ChannelStage{}, err
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return ChannelStage{}, fmt.Errorf("encode manifest: %w", err)
	}
	body = append(body, '\n')
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		return ChannelStage{}, err
	}
	return ChannelStage{
		Root:         root,
		ManifestPath: manifestPath,
		ArtifactPath: artPath,
	}, nil
}
