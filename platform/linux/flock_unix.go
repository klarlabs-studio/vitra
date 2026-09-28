//go:build unix

package linux

import (
	"os"
	"syscall"
)

// tryLock takes a non-blocking exclusive flock on f. held is false when
// another process already owns the lock.
func tryLock(f *os.File) (held bool, err error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
