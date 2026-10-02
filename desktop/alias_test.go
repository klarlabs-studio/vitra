package desktop_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
)

// A deny written with a folder's real path also applies when the file is
// reached through a symlinked spelling that the allow pattern uses.
func TestFileService_DenyOnRealPathAppliesThroughAlias(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(tmp, "data")
	if err := os.MkdirAll(filepath.Join(real, "secret"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"secret/key": "key", "notes.md": "notes"} {
		if err := os.WriteFile(filepath.Join(real, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(tmp, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.alias"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash
	grant, err := domain.NewCapabilityGrant("files", "files", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{
			Name: desktop.PermFSRead, PathScope: &domain.PathScope{
				Allow: []string{slash(alias) + "/**"},
				Deny:  []string{slash(real) + "/secret/**"},
			},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	caller, _ := rt.CallerFor("main")
	files := &desktop.FileService{Gateway: rt}

	_, err = files.Read(context.Background(), caller, filepath.Join(alias, "secret", "key"))
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialPathDenied {
		t.Fatalf("read through alias of a denied folder: %v", err)
	}
	if b, err := files.Read(context.Background(), caller, filepath.Join(alias, "notes.md")); err != nil || string(b) != "notes" {
		t.Fatalf("allowed read through alias = %q, %v", b, err)
	}
}
