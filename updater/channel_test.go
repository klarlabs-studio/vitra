package updater_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

func TestChannelSource_ManifestAndArtifactURL(t *testing.T) {
	src := updater.ChannelSource{
		BaseURL: "https://updates.example.com/vitra/",
		AppID:   "com.example.app",
		Channel: updater.ChannelStable,
	}
	mu, err := src.ManifestURL()
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := "https://updates.example.com/vitra/com.example.app/stable/manifest.json"
	if mu != wantManifest {
		t.Fatalf("manifest url=%q want %q", mu, wantManifest)
	}
	au, err := src.ArtifactURL("app.bin")
	if err != nil {
		t.Fatal(err)
	}
	wantArt := "https://updates.example.com/vitra/com.example.app/stable/app.bin"
	if au != wantArt {
		t.Fatalf("artifact url=%q want %q", au, wantArt)
	}
	abs, err := src.ArtifactURL("https://cdn.example.com/binaries/app.bin")
	if err != nil || abs != "https://cdn.example.com/binaries/app.bin" {
		t.Fatalf("absolute artifact=%q err=%v", abs, err)
	}
}

func TestChannelSource_RejectsBadInputs(t *testing.T) {
	cases := []updater.ChannelSource{
		{BaseURL: "file:///tmp", AppID: "a", Channel: "stable"},
		{BaseURL: "https://x", AppID: "../evil", Channel: "stable"},
		{BaseURL: "https://x", AppID: "a", Channel: "sta/ble"},
		{BaseURL: "", AppID: "a", Channel: "stable"},
	}
	for _, src := range cases {
		if _, err := src.ManifestURL(); err == nil {
			t.Fatalf("expected error for %+v", src)
		}
	}
	src := updater.ChannelSource{BaseURL: "https://x", AppID: "a", Channel: "stable"}
	for _, art := range []string{"../escape.bin", "/abs.bin", `..\win.bin`, ""} {
		if _, err := src.ArtifactURL(art); err == nil {
			t.Fatalf("expected artifact reject for %q", art)
		}
	}
}

func TestFetcher_FetchManifestAndArtifact(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("channel-binary-v3")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.example.app", Version: "3.0.0", Channel: updater.ChannelBeta,
		Artifact: "app.bin", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	manifestBody, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feed/com.example.app/beta/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(manifestBody)
	})
	mux.HandleFunc("/feed/com.example.app/beta/app.bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(artifact)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := updater.ChannelSource{
		BaseURL: srv.URL + "/feed/",
		AppID:   "com.example.app",
		Channel: updater.ChannelBeta,
	}
	f := &updater.Fetcher{Client: srv.Client(), MaxBytes: 1 << 20}
	got, err := f.FetchManifest(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.VerifyManifest(got, pub); err != nil {
		t.Fatal(err)
	}
	if got.Version != "3.0.0" {
		t.Fatalf("version=%q", got.Version)
	}
	body, err := f.FetchArtifact(context.Background(), src, got)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := updater.PlanInstall(got, pub, body)
	if err != nil || plan.Version != "3.0.0" {
		t.Fatalf("plan=%v err=%v", plan, err)
	}
}

func TestFetcher_HTTPErrorAndSizeLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a/stable/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	})
	mux.HandleFunc("/b/stable/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &updater.Fetcher{Client: srv.Client(), MaxBytes: 16}
	_, err := f.FetchManifest(context.Background(), updater.ChannelSource{
		BaseURL: srv.URL, AppID: "a", Channel: updater.ChannelStable,
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404, got %v", err)
	}
	_, err = f.FetchManifest(context.Background(), updater.ChannelSource{
		BaseURL: srv.URL, AppID: "b", Channel: updater.ChannelStable,
	})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected size limit, got %v", err)
	}
}
