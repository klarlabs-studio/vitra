package updater_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

func TestPlanInstall_RequiresValidSignatureAndDigest(t *testing.T) {
	// Security invariant 9.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("vitra-app-binary")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.example.app", Version: "1.2.3", Channel: updater.ChannelStable,
		Artifact: "app.appimage", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := updater.PlanInstall(m, pub, artifact)
	if err != nil || plan.Version != "1.2.3" {
		t.Fatalf("plan=%v err=%v", plan, err)
	}

	// Tampered artifact rejected.
	if _, err := updater.PlanInstall(m, pub, []byte("tampered")); err == nil {
		t.Fatal("expected digest failure")
	}
	// Bad signature rejected.
	bad := m
	bad.Signature = hex.EncodeToString(make([]byte, ed25519.SignatureSize))
	if _, err := updater.PlanInstall(bad, pub, artifact); err == nil {
		t.Fatal("expected signature failure")
	}
	// Unsigned rejected.
	unsigned := m
	unsigned.Signature = ""
	if _, err := updater.PlanInstall(unsigned, pub, artifact); err == nil {
		t.Fatal("expected missing signature failure")
	}
}
