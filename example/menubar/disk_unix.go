//go:build !windows

package main

import "syscall"

// diskSpace returns the size of the volume holding path and the space an
// unprivileged user can still write to it.
func diskSpace(path string) (total, free uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bsize := uint64(st.Bsize) //nolint:unconvert // int64 on Linux, uint32 on macOS
	return st.Blocks * bsize, st.Bavail * bsize, nil
}
