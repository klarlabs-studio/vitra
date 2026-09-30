// Package updater implements signed update verification.
//
// Update artifacts are never installed without integrity and authenticity
// checks (security invariant 9), and a validly signed manifest is still
// refused unless it is a newer release of the same app on the same channel,
// so old or foreign signed releases cannot be replayed.
package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	// ExpiresAt bounds how long the signed manifest is accepted, so a mirror
	// cannot keep serving an old release forever (freeze attack).
	ExpiresAt time.Time `json:"expires_at,omitzero"`
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
	// omitzero keeps manifests signed before expiry existed verifiable; they
	// are still refused by PlanInstall.
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

func (m Manifest) payloadBytes() ([]byte, error) {
	p := canonicalPayload{
		AppID: m.AppID, Version: m.Version, Channel: m.Channel,
		Artifact: m.Artifact, SHA256: m.SHA256, CreatedAt: m.CreatedAt.UTC(),
	}
	if !m.ExpiresAt.IsZero() {
		p.ExpiresAt = m.ExpiresAt.UTC()
	}
	return json.Marshal(p)
}

// DefaultManifestTTL is how long BuildSignedManifest makes a manifest valid.
const DefaultManifestTTL = 90 * 24 * time.Hour

// BuildSignedManifest constructs a manifest for artifact, digests it, and
// signs it, valid for DefaultManifestTTL.
func BuildSignedManifest(appID, version string, channel Channel, artifactName string, artifact []byte, priv ed25519.PrivateKey) (Manifest, error) {
	return BuildSignedManifestTTL(appID, version, channel, artifactName, artifact, priv, DefaultManifestTTL)
}

// BuildSignedManifestTTL is BuildSignedManifest with an explicit validity
// period. Re-sign (or re-stage) before it runs out, or clients stop
// accepting the release.
func BuildSignedManifestTTL(appID, version string, channel Channel, artifactName string, artifact []byte, priv ed25519.PrivateKey, ttl time.Duration) (Manifest, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(version) == "" {
		return Manifest{}, errors.New("app_id and version are required")
	}
	if ttl <= 0 {
		return Manifest{}, errors.New("manifest ttl must be positive")
	}
	if channel == "" {
		channel = ChannelStable
	}
	if channel != ChannelStable && channel != ChannelBeta {
		return Manifest{}, fmt.Errorf("unsupported channel %q (use stable or beta)", channel)
	}
	if strings.TrimSpace(artifactName) == "" {
		return Manifest{}, errors.New("artifact name is required")
	}
	now := time.Now().UTC()
	m := Manifest{
		AppID:     appID,
		Version:   version,
		Channel:   channel,
		Artifact:  artifactName,
		SHA256:    DigestArtifact(artifact),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	return SignManifest(m, priv)
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

// Errors returned by PlanInstall when a manifest is authentic but not an
// acceptable update for the installed app.
var (
	ErrWrongApp     = errors.New("update manifest is for a different app")
	ErrWrongChannel = errors.New("update manifest is for a different channel")
	ErrNotNewer     = errors.New("update version is not newer than the installed version")
	ErrExpired      = errors.New("update manifest is expired or has no expiry")
)

// Installed describes the app an update would replace.
type Installed struct {
	AppID   string
	Channel Channel
	// Version is the installed SemVer version. It is required: without it,
	// a replayed older release cannot be told apart from an upgrade.
	Version string
}

// PlanInstall verifies signature and digest, that the manifest has not
// expired, and that m is an update for installed: same app, same channel, and
// a strictly newer version. No
// plan is returned unless every check passes (invariant 9).
func PlanInstall(m Manifest, pub ed25519.PublicKey, artifact []byte, installed Installed) (InstallPlan, error) {
	if err := VerifyManifest(m, pub); err != nil {
		return InstallPlan{}, err
	}
	if err := VerifyArtifactDigest(m, artifact); err != nil {
		return InstallPlan{}, err
	}
	if m.ExpiresAt.IsZero() {
		return InstallPlan{}, fmt.Errorf("%w: manifest has no expires_at", ErrExpired)
	}
	if !m.ExpiresAt.After(m.CreatedAt) {
		return InstallPlan{}, errors.New("manifest expires_at is not after created_at")
	}
	if time.Now().After(m.ExpiresAt) {
		return InstallPlan{}, fmt.Errorf("%w: expired %s", ErrExpired, m.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if m.AppID != installed.AppID {
		return InstallPlan{}, fmt.Errorf("%w: manifest %q, installed %q", ErrWrongApp, m.AppID, installed.AppID)
	}
	if m.Channel != installed.Channel {
		return InstallPlan{}, fmt.Errorf("%w: manifest %q, installed %q", ErrWrongChannel, m.Channel, installed.Channel)
	}
	if installed.Version == "" {
		return InstallPlan{}, errors.New("installed version is required")
	}
	cmp, err := CompareVersions(m.Version, installed.Version)
	if err != nil {
		return InstallPlan{}, err
	}
	if cmp <= 0 {
		return InstallPlan{}, fmt.Errorf("%w: manifest %s, installed %s", ErrNotNewer, m.Version, installed.Version)
	}
	return InstallPlan{
		AppID: m.AppID, Version: m.Version, Channel: m.Channel,
		Artifact: m.Artifact, SHA256: m.SHA256,
	}, nil
}
