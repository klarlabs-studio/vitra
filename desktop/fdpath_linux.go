package desktop

import (
	"os"
	"strconv"
)

// fdPath returns the current path of the open file f.
func fdPath(f *os.File) (string, error) {
	p, err := os.Readlink("/proc/self/fd/" + strconv.FormatUint(uint64(f.Fd()), 10))
	if err != nil {
		// /proc is not mounted (some sandboxes).
		return "", errFdPathUnsupported
	}
	return p, nil
}
