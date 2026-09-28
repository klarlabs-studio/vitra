//go:build !unix

package darwin

import (
	"errors"
	"os"
)

// errFlockUnsupported is returned when this adapter is compiled for a non-unix
// OS (e.g. cross-built CLI tooling); the single-instance lock needs flock(2).
var errFlockUnsupported = errors.New("darwin: single-instance lock requires a unix host")

func tryLock(*os.File) (bool, error) { return false, errFlockUnsupported }

func unlock(*os.File) {}
