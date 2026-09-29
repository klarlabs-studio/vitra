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
// path with every symlink resolved.
//
// Path scopes are lexical, so a link inside the scope ("proj/escape ->
// /etc") would otherwise carry an allowed path anywhere on disk. After the
// lexical check, the real target must stay under the real scope root (so
// aliases above the root, such as macOS /var -> /private/var, are fine), and
// its location mapped back under the scope root is authorized again, so deny
// patterns also apply to where a link points.
//
// The check and the later file operation are not atomic: a process that can
// swap directories inside the scope between the two can still race it.
func authorizeRealPath(gw Gateway, caller domain.Caller, perm domain.PermissionName, path string) (string, error) {
	if gw == nil {
		return "", errGatewayRequired
	}
	d := gw.Authorize(caller, perm, path)
	if !d.Allowed {
		return "", deniedErr(caller, perm, d.Code, d.Reason)
	}
	if d.ScopeRoot == "" {
		// Not path-scoped: there is no boundary for a link to cross.
		return path, nil
	}
	root := filepath.FromSlash(d.ScopeRoot)
	realRoot, err := resolvePath(root)
	if err != nil {
		return "", deniedErr(caller, perm, domain.DenialPathOutOfScope, err.Error())
	}
	real, err := resolvePath(path)
	if err != nil {
		return "", deniedErr(caller, perm, domain.DenialPathOutOfScope, err.Error())
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", deniedErr(caller, perm, domain.DenialPathOutOfScope, "path resolves outside its scope through a symlink")
	}
	if mapped := filepath.Join(root, rel); mapped != filepath.Clean(path) {
		if d := gw.Authorize(caller, perm, mapped); !d.Allowed {
			return "", deniedErr(caller, perm, d.Code, "symlink target: "+d.Reason)
		}
	}
	return real, nil
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
