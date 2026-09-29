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
// on both the requested path and its symlink-resolved target, and the file
// operation acts on the resolved target.
type FileService struct {
	Gateway Gateway
	OnRead  func(ctx context.Context, path string) ([]byte, error)
	OnWrite func(ctx context.Context, path string, data []byte) error
}

// Read authorizes fs.read for path then reads the file.
func (s *FileService) Read(ctx context.Context, caller domain.Caller, path string) ([]byte, error) {
	real, err := authorizeRealPath(s.Gateway, caller, PermFSRead, path)
	if err != nil {
		return nil, err
	}
	if s.OnRead != nil {
		return s.OnRead(ctx, real)
	}
	return os.ReadFile(real)
}

// Write authorizes fs.write for path then writes data.
func (s *FileService) Write(ctx context.Context, caller domain.Caller, path string, data []byte) error {
	real, err := authorizeRealPath(s.Gateway, caller, PermFSWrite, path)
	if err != nil {
		return err
	}
	if s.OnWrite != nil {
		return s.OnWrite(ctx, real, data)
	}
	return os.WriteFile(real, data, 0o644)
}
