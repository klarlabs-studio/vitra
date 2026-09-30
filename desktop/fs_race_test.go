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

// raceFixture lays out a scope with a denied subtree and a directory outside
// it, all holding a file named f.txt:
//
//	<tmp>/proj/docs/f.txt        allowed
//	<tmp>/proj/.secrets/f.txt    denied
//	<tmp>/outside/f.txt          out of scope
func raceFixture(t *testing.T) (proj, outside string, files *desktop.FileService, caller domain.Caller) {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj, outside = filepath.Join(tmp, "proj"), filepath.Join(tmp, "outside")
	for dir, body := range map[string]string{
		filepath.Join(proj, "docs"):     "docs",
		filepath.Join(proj, ".secrets"): "secret",
		outside:                         "outside",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.race"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	scope := &domain.PathScope{
		Allow: []string{filepath.ToSlash(proj) + "/**"},
		Deny:  []string{filepath.ToSlash(proj) + "/.secrets/**"},
	}
	grant, err := domain.NewCapabilityGrant("files", "files", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{
			{Name: desktop.PermFSRead, PathScope: scope},
			{Name: desktop.PermFSWrite, PathScope: scope},
		})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	caller, _ = rt.CallerFor("main")
	return proj, outside, &desktop.FileService{Gateway: rt}, caller
}

// swapDocs replaces proj/docs with a symlink to target once the path has
// been authorized, as a racing local process could.
func swapDocs(t *testing.T, proj, target string) {
	t.Helper()
	restore := desktop.SetBeforeOpenHook(func() {
		docs := filepath.Join(proj, "docs")
		if err := os.Rename(docs, docs+".moved"); err != nil {
			t.Fatal(err)
		}
		// A relative link, as an attacker would use: os.Root follows it as
		// long as it stays inside the root.
		link := target
		if rel, err := filepath.Rel(filepath.Dir(docs), target); err == nil {
			link = rel
		}
		if err := os.Symlink(link, docs); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	})
	t.Cleanup(restore)
}

// The file operation acts on what was authorized: swapping a directory
// between the check and the open cannot redirect it.
func TestFileService_ReadCannotBeRacedOutOfScope(t *testing.T) {
	for name, target := range map[string]func(proj, outside string) string{
		"into the denied subtree": func(proj, _ string) string { return filepath.Join(proj, ".secrets") },
		"out of the scope":        func(_, outside string) string { return outside },
	} {
		t.Run(name, func(t *testing.T) {
			proj, outside, files, caller := raceFixture(t)
			swapDocs(t, proj, target(proj, outside))
			b, err := files.Read(context.Background(), caller, filepath.Join(proj, "docs", "f.txt"))
			if err == nil {
				t.Fatalf("read after swap returned %q", b)
			}
			var denied *domain.ErrDenied
			if !errors.As(err, &denied) {
				t.Fatalf("read after swap: %v, want a denial", err)
			}
		})
	}
}

func TestFileService_WriteCannotBeRacedOutOfScope(t *testing.T) {
	for name, target := range map[string]func(proj, outside string) string{
		"into the denied subtree": func(proj, _ string) string { return filepath.Join(proj, ".secrets") },
		"out of the scope":        func(_, outside string) string { return outside },
	} {
		for _, file := range []string{"f.txt", "new.txt"} {
			t.Run(name+"/"+file, func(t *testing.T) {
				proj, outside, files, caller := raceFixture(t)
				dest := target(proj, outside)
				swapDocs(t, proj, dest)
				err := files.Write(context.Background(), caller, filepath.Join(proj, "docs", file), []byte("pwned"))
				var denied *domain.ErrDenied
				if !errors.As(err, &denied) {
					t.Fatalf("write after swap: %v, want a denial", err)
				}
				if b, _ := os.ReadFile(filepath.Join(dest, file)); string(b) == "pwned" {
					t.Fatalf("write landed in %s", dest)
				}
				if file == "new.txt" {
					if _, err := os.Stat(filepath.Join(dest, file)); err == nil {
						t.Fatalf("an empty %s was left in %s", file, dest)
					}
				}
			})
		}
	}
}

// Without a race, reads and writes (including new files) still work.
func TestFileService_ReadWriteWithoutRace(t *testing.T) {
	proj, _, files, caller := raceFixture(t)
	ctx := context.Background()
	if b, err := files.Read(ctx, caller, filepath.Join(proj, "docs", "f.txt")); err != nil || string(b) != "docs" {
		t.Fatalf("read = %q, %v", b, err)
	}
	for _, name := range []string{"f.txt", "new.txt"} {
		p := filepath.Join(proj, "docs", name)
		if err := files.Write(ctx, caller, p, []byte("v2")); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if b, _ := os.ReadFile(p); string(b) != "v2" {
			t.Fatalf("%s = %q", name, b)
		}
	}
	// Overwriting with shorter content truncates.
	if err := files.Write(ctx, caller, filepath.Join(proj, "docs", "f.txt"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(proj, "docs", "f.txt")); string(b) != "x" {
		t.Fatalf("after shorter write: %q", b)
	}
}

// The same guarantees hold where the OS cannot say where an open file is.
func TestFileService_RaceWithoutFdPathLookup(t *testing.T) {
	t.Cleanup(desktop.ForceFdPathFallback())
	t.Run("read", TestFileService_ReadCannotBeRacedOutOfScope)
	t.Run("write", TestFileService_WriteCannotBeRacedOutOfScope)
	t.Run("no race", TestFileService_ReadWriteWithoutRace)
}
