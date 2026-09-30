//go:build !linux && !darwin && !windows

package desktop

import "os"

// fdPath is unsupported here; callers fall back to an identity check.
func fdPath(*os.File) (string, error) { return "", errFdPathUnsupported }
