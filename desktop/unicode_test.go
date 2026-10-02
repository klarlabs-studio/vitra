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

const (
	nfc = "Geheimnisse-é"  // é composed
	nfd = "Geheimnisse-é" // e + combining acute
)

// On filesystems that treat composed and decomposed Unicode spellings as one
// name (APFS), a deny written in one spelling must refuse the other.
func TestFileService_DenyIgnoresUnicodeSpelling(t *testing.T) {
	probe := t.TempDir()
	if err := os.Mkdir(filepath.Join(probe, nfd), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(probe, nfc)); err != nil {
		// Linux, Windows: the spellings name different files, so a deny on
		// one rightly does not cover the other.
		t.Skip("this filesystem keeps Unicode spellings apart; nothing to bypass")
	}
	for _, tc := range []struct{ stored, deny, request string }{
		{nfd, nfc, nfd},
		{nfc, nfd, nfc},
		{nfd, nfd, nfc},
		{nfc, nfc, nfd},
	} {
		tmp, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmp, tc.stored), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, tc.stored, "k.txt"), []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
		rt, err := vitra.New(vitra.Config{AppID: "com.example.unicode"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
			t.Fatal(err)
		}
		root := filepath.ToSlash(tmp)
		grant, err := domain.NewCapabilityGrant("files", "files", []domain.WindowID{"main"},
			[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{
				Name: desktop.PermFSRead, PathScope: &domain.PathScope{
					Allow: []string{root + "/**"},
					Deny:  []string{root + "/" + tc.deny + "/**"},
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
		b, err := files.Read(context.Background(), caller, filepath.Join(tmp, tc.request, "k.txt"))
		var denied *domain.ErrDenied
		if !errors.As(err, &denied) || denied.Code != domain.DenialPathDenied {
			t.Fatalf("stored %+q, deny %+q, request %+q: read = %q, %v", tc.stored, tc.deny, tc.request, b, err)
		}
	}
}
