package policy_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/policy"
)

func TestDocument_EncodeWritesSchema(t *testing.T) {
	if policy.DocumentSchema != "1" {
		t.Fatalf("DocumentSchema = %q", policy.DocumentSchema)
	}
	var buf bytes.Buffer
	if err := (policy.Document{}).Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "{\n  \"schema\": \"1\",\n") {
		t.Fatalf("encoded document does not lead with schema:\n%s", buf.String())
	}
	if _, err := policy.ParseDocument(&buf); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestDocument_SaveWritesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := (policy.Document{DenyPermissions: []domain.PermissionName{"shell.exec"}}).Save(path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"schema": "1"`) {
		t.Fatalf("saved document has no schema:\n%s", body)
	}
}

func TestParseDocument_RejectsUnknownSchema(t *testing.T) {
	for _, doc := range []string{
		`{"schema":"2","deny_permissions":["shell.exec"]}`,
		`{"schema":"0"}`,
		`{"schema":"v1"}`,
	} {
		_, err := policy.ParseDocument(strings.NewReader(doc))
		if !errors.Is(err, policy.ErrUnsupportedSchema) {
			t.Fatalf("%s: want ErrUnsupportedSchema, got %v", doc, err)
		}
	}
	if _, err := policy.ParseDocument(strings.NewReader(`{"schema":1}`)); err == nil {
		t.Fatal("numeric schema must be rejected")
	}
}

// A newer document may add fields this runtime does not know. It must be
// reported as an unsupported schema, not as an unknown field.
func TestParseDocument_UnknownSchemaWinsOverUnknownFields(t *testing.T) {
	_, err := policy.ParseDocument(strings.NewReader(`{"schema":"2","deny_hosts":["example.com"]}`))
	if !errors.Is(err, policy.ErrUnsupportedSchema) {
		t.Fatalf("want ErrUnsupportedSchema, got %v", err)
	}
}

func TestParseDocument_LegacyWithoutSchemaAcceptedAsV1(t *testing.T) {
	doc, err := policy.ParseDocument(strings.NewReader(`{"deny_permissions":["clipboard.read"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.DenyPermissions) != 1 || doc.DenyPermissions[0] != "clipboard.read" {
		t.Fatalf("%+v", doc)
	}
}

func TestParseDocument_V1(t *testing.T) {
	doc, err := policy.ParseDocument(strings.NewReader(`{"schema":"1","deny_permissions":["clipboard.read"],"require_update_signature":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.DenyPermissions) != 1 || !doc.RequireUpdateSignature {
		t.Fatalf("%+v", doc)
	}
}

func FuzzParseDocument(f *testing.F) {
	f.Add(`{"schema":"1","deny_permissions":["a"]}`)
	f.Add(`{"deny_permissions":[]}`)
	f.Add(`{"schema":"2","x":1}`)
	f.Add(`{"schema":null}`)
	f.Fuzz(func(t *testing.T, s string) {
		doc, err := policy.ParseDocument(strings.NewReader(s))
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := doc.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		if _, err := policy.ParseDocument(&buf); err != nil {
			t.Fatalf("re-encoded document does not parse: %v\n%s", err, buf.String())
		}
	})
}
