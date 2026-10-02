package fspath

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var procGetFinalPathNameByHandleW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// FdPath returns the current path of the open file f.
func FdPath(f *os.File) (string, error) {
	if err := procGetFinalPathNameByHandleW.Find(); err != nil {
		return "", ErrUnsupported
	}
	buf := make([]uint16, 512)
	for {
		// FILE_NAME_NORMALIZED | VOLUME_NAME_DOS == 0
		n, _, err := procGetFinalPathNameByHandleW.Call(f.Fd(), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		runtime.KeepAlive(f)
		if n == 0 {
			return "", err
		}
		if int(n) < len(buf) {
			p := syscall.UTF16ToString(buf[:n])
			switch {
			case strings.HasPrefix(p, `\\?\UNC\`):
				return `\\` + p[len(`\\?\UNC\`):], nil
			case strings.HasPrefix(p, `\\?\`):
				return p[len(`\\?\`):], nil
			}
			return p, nil
		}
		buf = make([]uint16, n)
	}
}
