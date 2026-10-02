package fspath

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// fGetPath is fcntl's F_GETPATH on Darwin; the buffer must hold MAXPATHLEN.
const (
	fGetPath   = 50
	maxPathLen = 1024
)

// FdPath returns the current path of the open file f.
func FdPath(f *os.File) (string, error) {
	var buf [maxPathLen]byte
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), fGetPath, uintptr(unsafe.Pointer(&buf[0])))
	runtime.KeepAlive(f)
	if errno != 0 {
		return "", errno
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf[:]), nil
}
