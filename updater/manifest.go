// Package updater implements Phase 4 signed update verification.
//
// Update artifacts are never installed without integrity and authenticity
// checks (security invariant 9).
package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Channel is a release channel name.
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// Manifest is a signed update description.
type Manifest struct {
	AppID     string    `json:"app_id"`
	Version   string    `json:"version"`
	Channel   Channel   `json:"channel"`
	Artifact  string    `json:"artifact"` // relative name
	SHA256    string    `json:"sha256"`   // hex
	CreatedAt time.Time `json:"created_at"`
	Signature string    `json:"signature"` // ed25519 over canonical payload, hex
}

// canonicalPayload is the signed bytes (signature field excluded).
type canonicalPayload struct {
	AppID     string    `json:"app_id"`
	Version   string    `json:"version"`
	Channel   Channel   `json:"channel"`
	Artifact  string    `json:"artifact"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
}

func (m Manifest) payloadBytes() ([]byte, error) {
	return json.Marshal(canonicalPayload{
		AppID: m.AppID, Version: m.Version, Channel: m.Channel,
		Artifact: m.Artifact, SHA256: m.SHA256, CreatedAt: m.CreatedAt.UTC(),
	})
}

// SignManifest signs a manifest with an ed25519 private key.
func SignManifest(m Manifest, priv ed25519.PrivateKey) (Manifest, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return m, errors.New("invalid private key size")
	}
	payload, err := m.payloadBytes()
	if err != nil {
		return m, err
	}
	sig := ed25519.Sign(priv, payload)
	m.Signature = hex.EncodeToString(sig)
	return m, nil
}

// VerifyManifest checks authenticity of a manifest against a public key.
func VerifyManifest(m Manifest, pub ed25519.PublicKey) error {
	if m.Signature == "" {
		return errors.New("manifest signature is required")
	}
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	sig, err := hex.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("signature encoding: %w", err)
	}
	payload, err := m.payloadBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, payload, sig) {
		return errors.New("manifest signature verification failed")
	}
	return nil
}

// VerifyArtifactDigest ensures artifact bytes match the manifest digest.
func VerifyArtifactDigest(m Manifest, artifact []byte) error {
	sum := sha256.Sum256(artifact)
	got := hex.EncodeToString(sum[:])
	if got != m.SHA256 {
		return fmt.Errorf("artifact digest mismatch: got %s want %s", got, m.SHA256)
	}
	return nil
}

// InstallPlan is produced only after verification succeeds.
type InstallPlan struct {
	AppID    string
	Version  string
	Channel  Channel
	Artifact string
	SHA256   string
}

// PlanInstall verifies signature + digest and returns an install plan.
// No plan is returned unless both checks pass (invariant 9).
func PlanInstall(m Manifest, pub ed25519.PublicKey, artifact []byte) (InstallPlan, error) {
	if err := VerifyManifest(m, pub); err != nil {
		return InstallPlan{}, err
	}
	if err := VerifyArtifactDigest(m, artifact); err != nil {
		return InstallPlan{}, err
	}
	return InstallPlan{
		AppID: m.AppID, Version: m.Version, Channel: m.Channel,
		Artifact: m.Artifact, SHA256: m.SHA256,
	}, nil
}
