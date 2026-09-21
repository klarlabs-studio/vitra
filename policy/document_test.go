package policy_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
)

func TestDocument_RoundTripJSON(t *testing.T) {
	doc := policy.Document{
		DenyPermissions:        []domain.PermissionName{"shell.exec", "clipboard.read"},
		AllowedUpdateChannels:  []updater.Channel{updater.ChannelStable},
		RequireUpdateSignature: true,
		DisableDevPrivileges:   true,
	}
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := policy.ParseDocument(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DenyPermissions) != 2 || got.DenyPermissions[0] != "shell.exec" {
		t.Fatalf("%+v", got)
	}
	if len(got.AllowedUpdateChannels) != 1 || got.AllowedUpdateChannels[0] != updater.ChannelStable {
		t.Fatalf("%+v", got)
	}
	if !got.RequireUpdateSignature || !got.DisableDevPrivileges {
		t.Fatalf("%+v", got)
	}
}

func TestLoadDocument_RejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"deny_permissions":[],"extra":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.LoadDocument(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("want unknown field error, got %v", err)
	}
}

func TestLoadSaveDocument_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet.json")
	doc := policy.Document{DenyPermissions: []domain.PermissionName{"shell.exec"}}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := policy.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DenyPermissions) != 1 || got.DenyPermissions[0] != "shell.exec" {
		t.Fatalf("%+v", got)
	}
}

func TestEngine_DocumentCopy(t *testing.T) {
	eng, err := policy.NewEngine(policy.Document{
		DenyPermissions: []domain.PermissionName{"shell.exec"},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	d := eng.Document()
	d.DenyPermissions[0] = "mutated"
	if eng.AllowsPermission("shell.exec") {
		t.Fatal("engine must retain original deny")
	}
	if !d.RequireUpdateSignature || !d.DisableDevPrivileges {
		t.Fatal("production Document() should reflect forced flags")
	}
}
