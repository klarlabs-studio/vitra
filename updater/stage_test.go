package updater_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

func TestStageChannel_WritesLayoutAndRejectsUnsigned(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("stage-binary")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.example.app", Version: "1.5.0", Channel: updater.ChannelStable,
		Artifact: "app.bin", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	stage, err := updater.StageChannel(out, m, artifact)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(out, "com.example.app", "stable")
	if stage.Root != wantRoot {
		t.Fatalf("root=%q want %q", stage.Root, wantRoot)
	}
	raw, err := os.ReadFile(stage.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var got updater.Manifest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := updater.VerifyManifest(got, pub); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(stage.ArtifactPath)
	if err != nil || string(body) != string(artifact) {
		t.Fatalf("artifact=%q err=%v", body, err)
	}

	unsigned := m
	unsigned.Signature = ""
	if _, err := updater.StageChannel(out, unsigned, artifact); err == nil {
		t.Fatal("expected unsigned reject")
	}
	if _, err := updater.StageChannel(out, m, []byte("tampered")); err == nil {
		t.Fatal("expected digest reject")
	}
}
