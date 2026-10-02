// Package fspath finds where files really are: the path of an open file as
// the OS reports it, and the canonical spelling of a directory.
package fspath

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrUnsupported is returned by FdPath where the OS cannot say where an open
// file is.
var ErrUnsupported = errors.New("open file path lookup unsupported")

// Canonical returns dir with symlinks resolved and, where the OS can report
// it, each name in the spelling stored on disk. macOS (APFS) treats composed
// and decomposed Unicode spellings of a name as the same file; the stored
// spelling is the one its open handles report, so comparing stored
// spellings sees through that. Elsewhere it equals filepath.EvalSymlinks.
func Canonical(dir string) (string, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	f, err := os.Open(real)
	if err != nil {
		return real, nil
	}
	defer func() { _ = f.Close() }()
	if fi, err := f.Stat(); err != nil || !fi.IsDir() {
		return real, nil
	}
	if p, err := FdPath(f); err == nil {
		return p, nil
	}
	return real, nil
}
