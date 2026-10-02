package fspath

import (
	"os"
	"strconv"
)

// FdPath returns the current path of the open file f.
func FdPath(f *os.File) (string, error) {
	p, err := os.Readlink("/proc/self/fd/" + strconv.FormatUint(uint64(f.Fd()), 10))
	if err != nil {
		// /proc is not mounted (some sandboxes).
		return "", ErrUnsupported
	}
	return p, nil
}
