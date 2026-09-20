package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// TrySingleInstance acquires an exclusive flock for appID under the user
// runtime directory. held is false when another instance already owns the lock.
// release closes the lock file; callers should defer it when held is true.
func (h *Host) TrySingleInstance(appID string) (held bool, release func(), err error) {
	if appID == "" {
		return false, nil, fmt.Errorf("app id required for single-instance lock")
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	// Use vitra-locks (not "vitra") to avoid colliding with a binary named /tmp/vitra.
	dir = filepath.Join(dir, "vitra-locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, nil, err
	}
	path := filepath.Join(dir, appID+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return false, func() {}, nil
		}
		return false, nil, err
	}
	release = func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return true, release, nil
}
