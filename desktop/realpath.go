package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/vitra/domain"
)

// authorizeRealPath authorizes perm for path and returns the path to act on:
// path with every symlink resolved. Callers that open the file themselves
// should use authorizeScoped and scopedPath.open instead, which cannot be
// raced.
func authorizeRealPath(gw Gateway, caller domain.Caller, perm domain.PermissionName, path string) (string, error) {
	sp, err := authorizeScoped(gw, caller, perm, path)
	return sp.real, err
}

// scopedPath is an authorized path. For path-scoped permissions it also
// records the scope root, so the file can be opened without leaving it.
type scopedPath struct {
	real     string // path with every symlink resolved
	scoped   bool   // the permission is path-scoped
	root     string // lexical scope root, as the grant names it
	realRoot string // scope root with symlinks resolved
	rel      string // real, relative to realRoot

	// Identities at authorization, for systems that cannot look up where an
	// open file is: the file (nil if it did not exist) and its directory.
	info       fs.FileInfo
	parentInfo fs.FileInfo
}

// authorizeScoped authorizes perm for path.
//
// Path scopes are lexical, so a link inside the scope ("proj/escape ->
// /etc") would otherwise carry an allowed path anywhere on disk. After the
// lexical check, the real target must stay under the real scope root (so
// aliases above the root, such as macOS /var -> /private/var, are fine), and
// its location mapped back under the scope root is authorized again, so deny
// patterns also apply to where a link points.
func authorizeScoped(gw Gateway, caller domain.Caller, perm domain.PermissionName, path string) (scopedPath, error) {
	if gw == nil {
		return scopedPath{}, errGatewayRequired
	}
	d := gw.Authorize(caller, perm, path)
	if !d.Allowed {
		return scopedPath{}, deniedErr(caller, perm, d.Code, d.Reason)
	}
	if d.ScopeRoot == "" {
		// Not path-scoped: there is no boundary for a link to cross.
		return scopedPath{real: path}, nil
	}
	root := filepath.FromSlash(d.ScopeRoot)
	realRoot, err := resolvePath(root)
	if err != nil {
		return scopedPath{}, deniedErr(caller, perm, domain.DenialPathOutOfScope, err.Error())
	}
	real, err := resolvePath(path)
	if err != nil {
		return scopedPath{}, deniedErr(caller, perm, domain.DenialPathOutOfScope, err.Error())
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return scopedPath{}, deniedErr(caller, perm, domain.DenialPathOutOfScope, "path resolves outside its scope through a symlink")
	}
	if mapped := filepath.Join(root, rel); mapped != filepath.Clean(path) {
		if d := gw.Authorize(caller, perm, mapped); !d.Allowed {
			return scopedPath{}, deniedErr(caller, perm, d.Code, "symlink target: "+d.Reason)
		}
	}
	sp := scopedPath{real: real, scoped: true, root: root, realRoot: realRoot, rel: rel}
	sp.info = statNow(real)
	sp.parentInfo = statNow(filepath.Dir(real))
	return sp, nil
}

// statNow returns path's FileInfo with its file identity read now. On
// Windows, os.Stat defers reading the identity until the first os.SameFile
// call, and reads it by path then; comparing later would see whatever the
// path points to by that time.
func statNow(path string) fs.FileInfo {
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	os.SameFile(fi, fi) // loads and caches the identity
	return fi
}

// resolvePath resolves every symlink in path. For a path that does not exist
// yet (a file about to be written), it resolves the deepest existing ancestor
// and appends the rest. A dangling link is an error: writing through it would
// create its target, wherever that is.
func resolvePath(path string) (string, error) {
	path = filepath.Clean(path)
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return real, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if fi, lerr := os.Lstat(path); lerr == nil && fi.Mode()&fs.ModeSymlink != 0 {
		return "", errors.New("path is a dangling symlink")
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}
	realParent, err := resolvePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(realParent, filepath.Base(path)), nil
}

func deniedErr(caller domain.Caller, perm domain.PermissionName, code domain.DenialCode, reason string) error {
	return &domain.ErrDenied{
		Permission: perm,
		Window:     caller.Window,
		Origin:     caller.Origin,
		Code:       code,
		Reason:     reason,
	}
}

// beforeOpen runs between authorization and the file operation (tests only).
var beforeOpen func()
