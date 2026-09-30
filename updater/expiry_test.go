package updater_test

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

var stable = updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.0.0"}

func signAt(t *testing.T, priv ed25519.PrivateKey, artifact []byte, created, expires time.Time) updater.Manifest {
	t.Helper()
	m := updater.Manifest{
		AppID: "com.example.app", Version: "1.1.0", Channel: updater.ChannelStable,
		Artifact: "app.bin", SHA256: updater.DigestArtifact(artifact),
		CreatedAt: created, ExpiresAt: expires,
	}
	m, err := updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A mirror that keeps serving an old manifest (a freeze attack) must stop
// working once the manifest expires, even though its signature stays valid.
func TestPlanInstall_RejectsExpiredOrUndatedManifests(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	artifact := []byte("v1.1.0")
	now := time.Now().UTC()

	fresh := signAt(t, priv, artifact, now.Add(-time.Hour), now.Add(24*time.Hour))
	if _, err := updater.PlanInstall(fresh, pub, artifact, stable); err != nil {
		t.Fatalf("fresh manifest rejected: %v", err)
	}
	expired := signAt(t, priv, artifact, now.Add(-48*time.Hour), now.Add(-time.Hour))
	if _, err := updater.PlanInstall(expired, pub, artifact, stable); !errors.Is(err, updater.ErrExpired) {
		t.Fatalf("expired: err=%v", err)
	}
	undated := signAt(t, priv, artifact, now, time.Time{})
	if _, err := updater.PlanInstall(undated, pub, artifact, stable); !errors.Is(err, updater.ErrExpired) {
		t.Fatalf("no expiry: err=%v", err)
	}
	inverted := signAt(t, priv, artifact, now.Add(time.Hour), now.Add(-time.Hour))
	if _, err := updater.PlanInstall(inverted, pub, artifact, stable); err == nil {
		t.Fatal("expiry before creation accepted")
	}
}

// Changing the expiry must invalidate the signature.
func TestManifest_ExpiryIsSigned(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now().UTC()
	m := signAt(t, priv, []byte("x"), now, now.Add(time.Hour))
	m.ExpiresAt = now.Add(365 * 24 * time.Hour)
	if err := updater.VerifyManifest(m, pub); err == nil {
		t.Fatal("extended expiry still verifies")
	}
}

func TestBuildSignedManifest_SetsDefaultExpiry(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	m, err := updater.BuildSignedManifest("com.example.app", "1.1.0", updater.ChannelStable, "app.bin", []byte("x"), priv)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.ExpiresAt.Sub(m.CreatedAt); got != updater.DefaultManifestTTL {
		t.Fatalf("ttl = %v, want %v", got, updater.DefaultManifestTTL)
	}
	m, err = updater.BuildSignedManifestTTL("com.example.app", "1.1.0", updater.ChannelStable, "app.bin", []byte("x"), priv, 72*time.Hour)
	if err != nil || m.ExpiresAt.Sub(m.CreatedAt) != 72*time.Hour {
		t.Fatalf("custom ttl: %v %v", m.ExpiresAt.Sub(m.CreatedAt), err)
	}
	if _, err := updater.BuildSignedManifestTTL("com.example.app", "1.1.0", updater.ChannelStable, "app.bin", []byte("x"), priv, 0); err == nil {
		t.Fatal("zero ttl accepted")
	}
}

// Callers must be able to tell a forged or tampered update apart from other
// failures without matching on message text.
func TestPlanInstall_TamperingHasSentinelErrors(t *testing.T) {
	keys, err := updater.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("release")
	m, err := updater.BuildSignedManifestTTL("com.example.app", "2.0.0", updater.ChannelStable, "app", artifact, keys.PrivateKey, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	installed := updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.0.0"}

	if _, err := updater.PlanInstall(m, keys.PublicKey, []byte("tampered"), installed); !errors.Is(err, updater.ErrDigestMismatch) {
		t.Fatalf("tampered artifact: %v, want updater.ErrDigestMismatch", err)
	}
	other, _ := updater.GenerateKeyPair()
	if _, err := updater.PlanInstall(m, other.PublicKey, artifact, installed); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("wrong key: %v, want updater.ErrBadSignature", err)
	}
	forged := m
	forged.Version = "9.0.0"
	if _, err := updater.PlanInstall(forged, keys.PublicKey, artifact, installed); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("edited manifest: %v, want updater.ErrBadSignature", err)
	}
	unsigned := m
	unsigned.Signature = ""
	if _, err := updater.PlanInstall(unsigned, keys.PublicKey, artifact, installed); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("unsigned manifest: %v, want updater.ErrBadSignature", err)
	}
}
