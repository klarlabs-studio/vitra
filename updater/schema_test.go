package updater_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/updater"
)

func signedV1(t *testing.T) (updater.Manifest, ed25519.PublicKey, []byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("notes 1.3.0")
	m, err := updater.BuildSignedManifest("com.example.app", "1.3.0", updater.ChannelStable, "notes", artifact, priv)
	if err != nil {
		t.Fatal(err)
	}
	return m, pub, artifact
}

func TestBuildSignedManifest_WritesSchema(t *testing.T) {
	m, pub, _ := signedV1(t)
	if updater.ManifestSchema != "1" || m.Schema != updater.ManifestSchema {
		t.Fatalf("schema = %q, want %q", m.Schema, updater.ManifestSchema)
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte(`{"schema":"1",`)) {
		t.Fatalf("encoded manifest does not lead with schema: %s", body)
	}
	got, err := updater.ParseManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != "1" {
		t.Fatalf("round trip lost schema: %+v", got)
	}
	if err := updater.VerifyManifest(got, pub); err != nil {
		t.Fatalf("round-tripped manifest must verify: %v", err)
	}
}

func TestSignManifest_StampsCurrentSchema(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	m, err := updater.SignManifest(updater.Manifest{AppID: "a", Version: "1.0.0", Channel: updater.ChannelStable}, priv)
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != updater.ManifestSchema {
		t.Fatalf("SignManifest wrote schema %q", m.Schema)
	}
}

func TestSignManifest_RejectsUnknownSchema(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	_, err := updater.SignManifest(updater.Manifest{Schema: "2", AppID: "a", Version: "1.0.0"}, priv)
	if !errors.Is(err, updater.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
}

func TestVerifyManifest_SignatureCoversSchema(t *testing.T) {
	m, pub, _ := signedV1(t)
	// Stripping the version must not keep a valid signature: it would turn
	// a schema 1 manifest into a legacy one.
	stripped := m
	stripped.Schema = ""
	if err := updater.VerifyManifest(stripped, pub); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("stripped schema: want ErrBadSignature, got %v", err)
	}
	// Removing it from the JSON must not verify either.
	body, _ := json.Marshal(m)
	edited := bytes.Replace(body, []byte(`"schema":"1",`), nil, 1)
	got, err := updater.ParseManifest(edited)
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.VerifyManifest(got, pub); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("removed schema: want ErrBadSignature, got %v", err)
	}
}

func TestVerifyManifest_RejectsUnknownSchema(t *testing.T) {
	m, pub, artifact := signedV1(t)
	for _, schema := range []string{"2", "0", "1.0", " 1", "v1"} {
		bad := m
		bad.Schema = schema
		if err := updater.VerifyManifest(bad, pub); !errors.Is(err, updater.ErrUnsupportedSchema) {
			t.Fatalf("schema %q: want ErrUnsupportedSchema, got %v", schema, err)
		}
		_, err := updater.PlanInstall(bad, pub, artifact, updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.0.0"})
		if !errors.Is(err, updater.ErrUnsupportedSchema) {
			t.Fatalf("PlanInstall schema %q: want ErrUnsupportedSchema, got %v", schema, err)
		}
	}
}

func TestParseManifest_RejectsUnknownSchema(t *testing.T) {
	_, err := updater.ParseManifest([]byte(`{"schema":"2","app_id":"a","version":"1.0.0"}`))
	if !errors.Is(err, updater.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
	if _, err := updater.ParseManifest([]byte(`{`)); err == nil || errors.Is(err, updater.ErrUnsupportedSchema) {
		t.Fatalf("malformed JSON: got %v", err)
	}
	if _, err := updater.ParseManifest([]byte(`{"schema":2}`)); err == nil {
		t.Fatal("numeric schema must be rejected")
	}
}

// Manifests signed before the schema field existed are read as schema 1, and
// their signatures still verify.
func TestManifest_LegacyWithoutSchemaAcceptedAsV1(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	artifact := []byte("legacy")
	created := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	expires := created.Add(time.Hour)
	// The exact bytes a 0.9 signer signed: no schema field.
	payload := `{"app_id":"com.example.app","version":"1.3.0","channel":"stable","artifact":"notes","sha256":"` +
		updater.DigestArtifact(artifact) + `","created_at":"` + created.Format(time.RFC3339Nano) +
		`","expires_at":"` + expires.Format(time.RFC3339Nano) + `"}`
	sig := ed25519.Sign(priv, []byte(payload))
	legacy := strings.TrimSuffix(payload, "}") + `,"signature":"` + hex.EncodeToString(sig) + `"}`

	m, err := updater.ParseManifest([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != "" {
		t.Fatalf("legacy manifest must keep an empty schema (it is not signed): %q", m.Schema)
	}
	if err := updater.VerifyManifest(m, pub); err != nil {
		t.Fatalf("legacy manifest must verify: %v", err)
	}
	plan, err := updater.PlanInstall(m, pub, artifact, updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.2.0"})
	if err != nil || plan.Version != "1.3.0" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	// Adding a schema to a legacy manifest is tampering.
	m.Schema = updater.ManifestSchema
	if err := updater.VerifyManifest(m, pub); !errors.Is(err, updater.ErrBadSignature) {
		t.Fatalf("added schema: want ErrBadSignature, got %v", err)
	}
}

func TestFetchManifest_RejectsUnknownSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schema":"9","app_id":"com.example.app","channel":"stable"}`))
	}))
	defer srv.Close()
	f := &updater.Fetcher{Client: srv.Client()}
	_, err := f.FetchManifest(t.Context(), updater.ChannelSource{BaseURL: srv.URL, AppID: "com.example.app", Channel: updater.ChannelStable})
	if !errors.Is(err, updater.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
}

func TestStageChannel_RejectsUnknownSchema(t *testing.T) {
	m, _, artifact := signedV1(t)
	m.Schema = "2"
	if _, err := updater.StageChannel(t.TempDir(), m, artifact); !errors.Is(err, updater.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
}

func FuzzParseManifest(f *testing.F) {
	f.Add([]byte(`{"schema":"1","app_id":"a","version":"1.0.0","channel":"stable"}`))
	f.Add([]byte(`{"app_id":"a"}`))
	f.Add([]byte(`{"schema":"2"}`))
	f.Add([]byte(`{"schema":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := updater.ParseManifest(data)
		if err != nil {
			return
		}
		if m.Schema != "" && m.Schema != updater.ManifestSchema {
			t.Fatalf("accepted unknown schema %q", m.Schema)
		}
	})
}
