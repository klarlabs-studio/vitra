package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/vitra/domain"
)

// errFdPathUnsupported is returned by fdPath where the OS cannot say where
// an open file is.
var errFdPathUnsupported = errors.New("open file path lookup unsupported")

// lookupFdPath is fdPath; tests replace it to exercise the fallback.
var lookupFdPath = fdPath

// open opens the existing file at sp for reading or writing so that nothing
// done to the filesystem after authorization can redirect it:
//
//  1. The file is opened through an os.Root at the real scope root, so the
//     kernel refuses any path that leaves the scope, however directories
//     were swapped in the meantime.
//  2. The opened file's actual location is looked up from the handle and
//     authorized again, so a swap within the scope cannot reach a denied
//     path either.
//
// All I/O then goes through the returned handle, which cannot be
// redirected.
func (sp scopedPath) open(gw Gateway, caller domain.Caller, perm domain.PermissionName, flag int) (*os.File, error) {
	root, err := os.OpenRoot(sp.realRoot)
	if err != nil {
		return nil, deniedErr(caller, perm, domain.DenialPathOutOfScope, "scope root: "+err.Error())
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile(sp.rel, flag, 0o644)
	if err != nil {
		return nil, openErr(caller, perm, err)
	}
	if err := sp.verify(gw, caller, perm, f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// create creates the file at sp, which did not exist at authorization. The
// parent directory is opened through the scope root and must be the one
// identified at authorization; the file is then created exclusively inside
// that directory handle, so neither a swapped directory nor a planted link
// can move it. Nothing is created anywhere else.
func (sp scopedPath) create(gw Gateway, caller domain.Caller, perm domain.PermissionName) (*os.File, error) {
	root, err := os.OpenRoot(sp.realRoot)
	if err != nil {
		return nil, deniedErr(caller, perm, domain.DenialPathOutOfScope, "scope root: "+err.Error())
	}
	defer func() { _ = root.Close() }()
	dir, err := root.OpenRoot(filepath.Dir(sp.rel))
	if err != nil {
		return nil, openErr(caller, perm, err)
	}
	defer func() { _ = dir.Close() }()
	cur, err := dir.Stat(".")
	if err != nil || sp.parentInfo == nil || !os.SameFile(cur, sp.parentInfo) {
		return nil, deniedErr(caller, perm, domain.DenialPathOutOfScope, "directory changed after it was authorized")
	}
	f, err := dir.OpenFile(filepath.Base(sp.rel), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, openErr(caller, perm, err)
	}
	if err := sp.verify(gw, caller, perm, f); err != nil {
		removeOpened(f)
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// openErr turns an os.Root error into a denial when the path left the root.
func openErr(caller domain.Caller, perm domain.PermissionName, err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrExist) || errors.Is(err, fs.ErrPermission) {
		return err
	}
	return deniedErr(caller, perm, domain.DenialPathOutOfScope, "path leaves its scope: "+err.Error())
}

// verify authorizes where f actually is.
func (sp scopedPath) verify(gw Gateway, caller domain.Caller, perm domain.PermissionName, f *os.File) error {
	actual, err := lookupFdPath(f)
	if errors.Is(err, errFdPathUnsupported) {
		return sp.verifyIdentity(caller, perm, f)
	}
	if err != nil {
		return deniedErr(caller, perm, domain.DenialPathOutOfScope, "locate opened file: "+err.Error())
	}
	rel, err := filepath.Rel(sp.realRoot, actual)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return deniedErr(caller, perm, domain.DenialPathOutOfScope, "opened file is outside its scope")
	}
	if d := gw.Authorize(caller, perm, filepath.Join(sp.root, rel)); !d.Allowed {
		return deniedErr(caller, perm, d.Code, "opened file: "+d.Reason)
	}
	return nil
}

// verifyIdentity is verify for systems that cannot say where an open file
// is. An existing file must be the one identified at authorization. A new
// file was created inside the directory handle checked by create, so it is
// where it was authorized to be.
func (sp scopedPath) verifyIdentity(caller domain.Caller, perm domain.PermissionName, f *os.File) error {
	if sp.info == nil {
		return nil
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(opened, sp.info) {
		return deniedErr(caller, perm, domain.DenialPathOutOfScope, "file changed after it was authorized")
	}
	return nil
}

// removeOpened deletes a file this process just created in the wrong place,
// if it is still there.
func removeOpened(f *os.File) {
	p, err := lookupFdPath(f)
	if err != nil {
		return
	}
	opened, err := f.Stat()
	if err != nil {
		return
	}
	if cur, err := os.Lstat(p); err == nil && os.SameFile(opened, cur) {
		_ = os.Remove(p)
	}
}
