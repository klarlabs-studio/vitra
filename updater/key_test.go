package updater_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/updater"
)

func TestGenerateAndWriteKeyPair(t *testing.T) {
	kp, err := updater.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if len(kp.PrivateKey) != ed25519.PrivateKeySize || len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("key sizes priv=%d pub=%d", len(kp.PrivateKey), len(kp.PublicKey))
	}
	dir := t.TempDir()
	privPath, pubPath, err := updater.WriteKeyPair(dir, kp)
	if err != nil {
		t.Fatal(err)
	}
	privBody, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(privPath); err != nil || st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("priv.key mode=%v err=%v", st.Mode(), err)
	}
	if strings.TrimSpace(string(privBody)) != kp.PrivateHex {
		t.Fatalf("priv file mismatch")
	}
	pubBody, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(pubBody)) != kp.PublicHex {
		t.Fatalf("pub file mismatch")
	}
}

func TestLoadPrivateKeyRef(t *testing.T) {
	kp, err := updater.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_UPDATE_PRIV", kp.PrivateHex)
	got, err := updater.LoadPrivateKeyRef("env:VITRA_UPDATE_PRIV")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(kp.PrivateKey) {
		t.Fatal("env load mismatch")
	}

	path := filepath.Join(t.TempDir(), "k.hex")
	if err := os.WriteFile(path, []byte(kp.PrivateHex+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = updater.LoadPrivateKeyRef("file:" + path)
	if err != nil || !got.Equal(kp.PrivateKey) {
		t.Fatalf("file load: %v", err)
	}

	t.Setenv("VITRA_SECRET_CI_UPDATE", kp.PrivateHex)
	got, err = updater.LoadPrivateKeyRef("secret:ci/update")
	if err != nil || !got.Equal(kp.PrivateKey) {
		t.Fatalf("secret load: %v", err)
	}

	if _, err := updater.LoadPrivateKeyRef(kp.PrivateHex); err == nil {
		t.Fatal("expected bare hex reject")
	}
	if _, err := updater.LoadPrivateKeyRef("env:MISSING_UPDATE_KEY"); err == nil {
		t.Fatal("expected missing env")
	}
	_ = os.Unsetenv("VITRA_SECRET_CI_MISSING")
	if _, err := updater.LoadPrivateKeyRef("secret:ci/missing"); err == nil {
		t.Fatal("expected missing secret")
	}
}

func TestBuildSignedManifest(t *testing.T) {
	kp, err := updater.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("signed-bytes")
	m, err := updater.BuildSignedManifest("com.example.app", "2.0.0", updater.ChannelBeta, "app.bin", artifact, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if m.SHA256 != updater.DigestArtifact(artifact) {
		t.Fatalf("digest=%s", m.SHA256)
	}
	if err := updater.VerifyManifest(m, kp.PublicKey); err != nil {
		t.Fatal(err)
	}
	if _, err := updater.BuildSignedManifest("", "1", updater.ChannelStable, "a", artifact, kp.PrivateKey); err == nil {
		t.Fatal("expected empty app_id reject")
	}
	if _, err := updater.BuildSignedManifest("a", "1", "nightly", "a", artifact, kp.PrivateKey); err == nil {
		t.Fatal("expected bad channel reject")
	}
	_ = hex.EncodeToString // keep import used if DigestArtifact path changes
}
