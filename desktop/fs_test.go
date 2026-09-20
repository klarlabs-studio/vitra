package desktop_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
)

func TestFileService_PathScopeEnforced(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.fs"})
	if err != nil {
		t.Fatal(err)
	}
	caller, _ := domain.NewCaller("main", domain.OriginPackagedLocal)
	grant, err := domain.NewCapabilityGrant(
		"files", "files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{
			Name: desktop.PermFSRead,
			PathScope: &domain.PathScope{
				Allow: []string{"/project/**"},
				Deny:  []string{"/project/.secrets/**"},
			},
		}, {
			Name: desktop.PermFSWrite,
			PathScope: &domain.PathScope{
				Allow: []string{"/project/**"},
				Deny:  []string{"/project/.secrets/**"},
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}

	var readPath, writePath string
	files := &desktop.FileService{
		Gateway: rt,
		OnRead: func(_ context.Context, path string) ([]byte, error) {
			readPath = path
			return []byte("ok"), nil
		},
		OnWrite: func(_ context.Context, path string, data []byte) error {
			writePath = path
			if string(data) != "hi" {
				t.Fatalf("data=%q", data)
			}
			return nil
		},
	}

	if _, err := files.Read(context.Background(), caller, "/project/a.go"); err != nil {
		t.Fatal(err)
	}
	if readPath != "/project/a.go" {
		t.Fatalf("readPath=%q", readPath)
	}
	if err := files.Write(context.Background(), caller, "/project/out.txt", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if writePath != "/project/out.txt" {
		t.Fatalf("writePath=%q", writePath)
	}

	_, err = files.Read(context.Background(), caller, "/project/.secrets/token")
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) {
		t.Fatalf("expected deny for secrets, got %v", err)
	}
	_, err = files.Read(context.Background(), caller, "/project/../etc/passwd")
	if !errors.As(err, &denied) {
		t.Fatalf("expected deny for traversal, got %v", err)
	}
	_, err = files.Read(context.Background(), caller, "/other/file")
	if !errors.As(err, &denied) {
		t.Fatalf("expected deny for out of scope, got %v", err)
	}
	if readPath != "/project/a.go" {
		t.Fatal("OnRead must not run for denied paths")
	}
}
