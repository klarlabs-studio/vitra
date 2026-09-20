package updater_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

func TestApplyInstall_AtomicReplace(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("vitra-app-binary-v2")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.example.app", Version: "2.0.0", Channel: updater.ChannelStable,
		Artifact: "app.bin", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := updater.PlanInstall(m, pub, artifact)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "app.bin")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := updater.ApplyInstall(plan, artifact, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(artifact) {
		t.Fatalf("dest content = %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected executable bits, mode=%v", info.Mode())
	}

	// Tampered bytes must not replace dest.
	if err := updater.ApplyInstall(plan, []byte("tampered"), dest); err == nil {
		t.Fatal("expected digest failure")
	}
	got, _ = os.ReadFile(dest)
	if string(got) != string(artifact) {
		t.Fatalf("dest should remain verified artifact, got %q", got)
	}
}

func TestApplyInstall_RejectsIncompletePlan(t *testing.T) {
	if err := updater.ApplyInstall(updater.InstallPlan{}, []byte("x"), filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("expected incomplete plan error")
	}
}
