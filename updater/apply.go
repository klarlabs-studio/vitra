package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// staleSuffix marks a replaced install that could not be removed yet (for
// example a Windows executable that is still running).
const staleSuffix = ".vitra-old-"

var (
	// moveAsideFirst replaces a file by renaming the existing one aside before
	// moving the new one in. Windows needs this: a running executable can be
	// renamed but not overwritten.
	moveAsideFirst = runtime.GOOS == "windows"

	// maxExtractedBytes bounds the total size an archive may unpack to.
	maxExtractedBytes int64 = 2 << 30 // 2 GiB
)

// maxArchiveEntries bounds the number of entries an archive may contain.
const maxArchiveEntries = 200_000

// ApplyInstall installs a verified artifact at destPath.
//
// A plain artifact (a binary) replaces the file at destPath. An archive
// (plan.Artifact ending in .tar.gz, .tgz, or .zip) replaces the directory at
// destPath, such as a macOS .app bundle; when the archive holds exactly one
// top-level directory, that directory's contents become destPath.
//
// The new version is fully written next to destPath before anything is
// replaced, and a failed install leaves the existing one untouched. Archive
// entries that are absolute, climb out with "..", or are symlinks pointing
// outside the archive are rejected. Replaced installs that cannot be deleted
// yet (a running Windows executable) are left as "<dest>.vitra-old-*";
// call CleanupStale at startup to remove them.
//
// Callers must obtain plan from PlanInstall (invariant 9); the digest is
// re-checked here before any filesystem mutation.
func ApplyInstall(plan InstallPlan, artifact []byte, destPath string) error {
	if plan.AppID == "" || plan.Version == "" || plan.SHA256 == "" {
		return errors.New("install plan is incomplete")
	}
	if destPath == "" {
		return errors.New("destination path is required")
	}
	if err := VerifyArtifactDigest(Manifest{SHA256: plan.SHA256}, artifact); err != nil {
		return err
	}
	destPath = filepath.Clean(destPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	switch archiveKind(plan.Artifact) {
	case "zip", "tar.gz":
		return installArchive(archiveKind(plan.Artifact), artifact, destPath)
	default:
		return installFile(artifact, destPath)
	}
}

func archiveKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	}
	return ""
}

func installFile(artifact []byte, destPath string) error {
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".vitra-update-*")
	if err != nil {
		return fmt.Errorf("create temp artifact: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once renamed

	if _, err := tmp.Write(artifact); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp artifact: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp artifact: %w", err)
	}
	if !moveAsideFirst {
		if err := os.Rename(tmpName, destPath); err != nil {
			return fmt.Errorf("replace %s: %w", destPath, err)
		}
		return nil
	}
	return swapIn(tmpName, destPath)
}

func installArchive(kind string, artifact []byte, destPath string) error {
	staging, err := os.MkdirTemp(filepath.Dir(destPath), ".vitra-update-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }() // no-op once swapped in

	if kind == "zip" {
		err = extractZip(artifact, staging)
	} else {
		err = extractTarGz(artifact, staging)
	}
	if err != nil {
		return fmt.Errorf("extract update: %w", err)
	}
	root, err := bundleRoot(staging)
	if err != nil {
		return err
	}
	return swapIn(root, destPath)
}

// bundleRoot returns the single top-level directory of an extracted archive,
// or the staging directory itself when the archive has loose entries.
func bundleRoot(staging string) (string, error) {
	entries, err := os.ReadDir(staging)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", errors.New("update archive is empty")
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(staging, entries[0].Name()), nil
	}
	return staging, nil
}

// swapIn replaces destPath with newPath. An existing destPath is renamed
// aside first, restored if the swap fails, and removed afterwards when
// possible.
func swapIn(newPath, destPath string) error {
	if _, err := os.Lstat(destPath); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(newPath, destPath)
	}
	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	old := destPath + staleSuffix + suffix
	if err := os.Rename(destPath, old); err != nil {
		return fmt.Errorf("move current install aside: %w", err)
	}
	if err := os.Rename(newPath, destPath); err != nil {
		if rerr := os.Rename(old, destPath); rerr != nil {
			return fmt.Errorf("install new version: %w (restoring the previous version also failed: %v; it is at %s)", err, rerr, old)
		}
		return fmt.Errorf("install new version: %w", err)
	}
	// Best effort: a running executable cannot be deleted on Windows.
	_ = os.RemoveAll(old)
	return nil
}

// CleanupStale removes replaced installs left next to destPath by a previous
// ApplyInstall that could not delete them while they were in use. Call it
// early at startup, before anything reopens the old files.
func CleanupStale(destPath string) error {
	destPath = filepath.Clean(destPath)
	matches, err := filepath.Glob(destPath + staleSuffix + "*")
	if err != nil {
		return err
	}
	var errs []error
	for _, m := range matches {
		if err := os.RemoveAll(m); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func randomSuffix() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// safeJoin maps an archive entry name to a path under root, rejecting
// absolute names and any ".." segment (with either separator).
func safeJoin(root, name string) (string, error) {
	slashed := strings.ReplaceAll(name, `\`, "/")
	if slashed == "" || path.IsAbs(slashed) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("archive entry %q is not a relative path", name)
	}
	for _, seg := range strings.Split(slashed, "/") {
		if seg == ".." {
			return "", fmt.Errorf("archive entry %q escapes the archive", name)
		}
	}
	return filepath.Join(root, filepath.FromSlash(path.Clean(slashed))), nil
}

// checkLink rejects symlinks whose target is absolute or resolves outside
// root relative to the link's own directory.
func checkLink(root, linkPath, target string) error {
	if target == "" || path.IsAbs(target) || filepath.IsAbs(target) || filepath.VolumeName(target) != "" {
		return fmt.Errorf("symlink %q -> %q must be relative", linkPath, target)
	}
	resolved := filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(strings.ReplaceAll(target, `\`, "/")))
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("symlink %q -> %q points outside the archive", linkPath, target)
	}
	return nil
}

// budget tracks extracted size and entry count against the limits.
type budget struct {
	bytes   int64
	entries int
}

func (b *budget) entry() error {
	b.entries++
	if b.entries > maxArchiveEntries {
		return fmt.Errorf("archive has more than %d entries", maxArchiveEntries)
	}
	return nil
}

// copyLimited copies r to a new file at p, charging the budget.
func (b *budget) copyLimited(p string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm()|0o600)
	if err != nil {
		return err
	}
	remaining := maxExtractedBytes - b.bytes
	n, err := io.Copy(f, io.LimitReader(r, remaining+1))
	b.bytes += n
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > remaining {
		return fmt.Errorf("archive unpacks to more than %d bytes", maxExtractedBytes)
	}
	return nil
}

func extractZip(artifact []byte, root string) error {
	zr, err := zip.NewReader(bytes.NewReader(artifact), int64(len(artifact)))
	if err != nil {
		return err
	}
	var b budget
	for _, f := range zr.File {
		if err := b.entry(); err != nil {
			return err
		}
		p, err := safeJoin(root, f.Name)
		if err != nil {
			return err
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			target, err := io.ReadAll(io.LimitReader(rc, 4096))
			_ = rc.Close()
			if err != nil {
				return err
			}
			if err := makeLink(root, p, string(target)); err != nil {
				return err
			}
		case mode.IsRegular():
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = b.copyLimited(p, rc, mode)
			_ = rc.Close()
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive entry %q has unsupported type %v", f.Name, mode.Type())
		}
	}
	return nil
}

func extractTarGz(artifact []byte, root string) error {
	gz, err := gzip.NewReader(bytes.NewReader(artifact))
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var b budget
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := b.entry(); err != nil {
			return err
		}
		p, err := safeJoin(root, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := makeLink(root, p, h.Linkname); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := b.copyLimited(p, tr, os.FileMode(h.Mode)); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader:
			// PAX metadata, nothing to extract.
		default:
			return fmt.Errorf("archive entry %q has unsupported type %q", h.Name, h.Typeflag)
		}
	}
}

func makeLink(root, p, target string) error {
	if err := checkLink(root, p, target); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, p)
}
