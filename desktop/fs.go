package desktop

import (
	"context"
	"io"
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
	sp, err := authorizeScoped(s.Gateway, caller, PermFSRead, path)
	if err != nil {
		return nil, err
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	if s.OnRead != nil {
		return s.OnRead(ctx, sp.real)
	}
	if !sp.scoped {
		return os.ReadFile(sp.real)
	}
	f, err := sp.open(s.Gateway, caller, PermFSRead, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// Write authorizes fs.write for path then writes data, creating the file if
// needed.
func (s *FileService) Write(ctx context.Context, caller domain.Caller, path string, data []byte) error {
	sp, err := authorizeScoped(s.Gateway, caller, PermFSWrite, path)
	if err != nil {
		return err
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	if s.OnWrite != nil {
		return s.OnWrite(ctx, sp.real, data)
	}
	if !sp.scoped {
		return os.WriteFile(sp.real, data, 0o644)
	}
	// Open without truncating, so nothing is lost if the opened file turns
	// out not to be the authorized one. A file that did not exist is created
	// exclusively in the authorized directory.
	var f *os.File
	if sp.info != nil {
		f, err = sp.open(s.Gateway, caller, PermFSWrite, os.O_WRONLY)
	} else {
		f, err = sp.create(s.Gateway, caller, PermFSWrite)
	}
	if err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
