package desktop

import (
	"context"
	"os"

	"go.klarlabs.de/vitra/domain"
)

// Filesystem permission names owned by the official vitra.fs plugin.
const (
	PermFSRead  domain.PermissionName = "fs.read"
	PermFSWrite domain.PermissionName = "fs.write"
)

// FileService provides grant-scoped filesystem access for host-bound
// vitra.fs plugin commands. PathScope is enforced via the capability gateway
// using the same path that will be read or written.
type FileService struct {
	Gateway Gateway
	OnRead  func(ctx context.Context, path string) ([]byte, error)
	OnWrite func(ctx context.Context, path string, data []byte) error
}

// Read authorizes fs.read for path then reads the file.
func (s *FileService) Read(ctx context.Context, caller domain.Caller, path string) ([]byte, error) {
	if err := authorizePath(s.Gateway, caller, PermFSRead, path); err != nil {
		return nil, err
	}
	if s.OnRead != nil {
		return s.OnRead(ctx, path)
	}
	return os.ReadFile(path)
}

// Write authorizes fs.write for path then writes data.
func (s *FileService) Write(ctx context.Context, caller domain.Caller, path string, data []byte) error {
	if err := authorizePath(s.Gateway, caller, PermFSWrite, path); err != nil {
		return err
	}
	if s.OnWrite != nil {
		return s.OnWrite(ctx, path, data)
	}
	return os.WriteFile(path, data, 0o644)
}
