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

// symlinkFixture lays out:
//
//	<tmp>/outside/secret.txt           outside the scope
//	<tmp>/proj/ok.txt                  plain file in scope
//	<tmp>/proj/.secrets/key            denied subtree
//	<tmp>/proj/escape    -> ../outside directory link out of scope
//	<tmp>/proj/leak.txt  -> ../outside/secret.txt
//	<tmp>/proj/public    -> .secrets   link into the denied subtree
//	<tmp>/proj/alias.txt -> ok.txt     link that stays in scope
//	<tmp>/proj/dangling  -> ../outside/created.txt (does not exist yet)
//
// t.TempDir is itself behind a symlink on macOS (/var -> /private/var), so
// these tests also check that aliases above the scope root are tolerated.
func symlinkFixture(t *testing.T) (proj, outside string, files *desktop.FileService, caller domain.Caller) {
	t.Helper()
	tmp := t.TempDir()
	proj = filepath.Join(tmp, "proj")
	outside = filepath.Join(tmp, "outside")
	for _, dir := range []string{outside, filepath.Join(proj, ".secrets")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(outside, "secret.txt"):   "outside",
		filepath.Join(proj, "ok.txt"):          "ok",
		filepath.Join(proj, ".secrets", "key"): "key",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"escape":    filepath.Join("..", "outside"),
		"leak.txt":  filepath.Join("..", "outside", "secret.txt"),
		"public":    ".secrets",
		"alias.txt": "ok.txt",
		"dangling":  filepath.Join("..", "outside", "created.txt"),
	} {
		if err := os.Symlink(target, filepath.Join(proj, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	rt, err := vitra.New(vitra.Config{AppID: "com.example.fs"})
	if err != nil {
		t.Fatal(err)
	}
	scope := &domain.PathScope{
		Allow: []string{proj + "/**"},
		Deny:  []string{proj + "/.secrets/**"},
	}
	grant, err := domain.NewCapabilityGrant("files", "files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: desktop.PermFSRead, PathScope: scope},
			{Name: desktop.PermFSWrite, PathScope: scope},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	caller, _ = domain.NewCaller("main", domain.OriginPackagedLocal)
	return proj, outside, &desktop.FileService{Gateway: rt}, caller
}

func TestFileService_ReadFollowsSymlinksOnlyWithinScope(t *testing.T) {
	proj, _, files, caller := symlinkFixture(t)
	ctx := context.Background()

	for _, name := range []string{"ok.txt", "alias.txt"} {
		data, err := files.Read(ctx, caller, filepath.Join(proj, name))
		if err != nil || string(data) != "ok" {
			t.Fatalf("read %s: %q %v", name, data, err)
		}
	}
	for _, rel := range []string{
		filepath.Join("escape", "secret.txt"), // directory link out of scope
		"leak.txt",                            // file link out of scope
		filepath.Join("public", "key"),        // link into the denied subtree
	} {
		data, err := files.Read(ctx, caller, filepath.Join(proj, rel))
		var denied *domain.ErrDenied
		if !errors.As(err, &denied) {
			t.Fatalf("read %s: want denial, got %q %v", rel, data, err)
		}
	}
}

func TestFileService_WriteCannotEscapeThroughSymlinks(t *testing.T) {
	proj, outside, files, caller := symlinkFixture(t)
	ctx := context.Background()

	if err := files.Write(ctx, caller, filepath.Join(proj, "new.txt"), []byte("hi")); err != nil {
		t.Fatalf("plain write: %v", err)
	}
	for _, rel := range []string{
		filepath.Join("escape", "created.txt"),
		"dangling",
		filepath.Join("public", "planted"),
	} {
		err := files.Write(ctx, caller, filepath.Join(proj, rel), []byte("x"))
		var denied *domain.ErrDenied
		if !errors.As(err, &denied) {
			t.Fatalf("write %s: want denial, got %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "created.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("write escaped the scope: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".secrets", "planted")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("write reached the denied subtree: %v", err)
	}
}
