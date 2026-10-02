//go:build !linux && !darwin && !windows

package fspath

import "os"

// FdPath is unsupported here; callers fall back to an identity check.
func FdPath(*os.File) (string, error) { return "", ErrUnsupported }
