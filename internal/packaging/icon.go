package packaging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// stageIconFile copies spec.IconPath into destDir as <baseName><ext>.
// Returns the FreeDesktop Icon= key (baseName, no extension) and the written
// file name (with extension). Empty IconPath is a no-op.
func stageIconFile(iconPath, destDir, baseName string) (iconKey, fileName string, err error) {
	if strings.TrimSpace(iconPath) == "" {
		return "", "", nil
	}
	src, err := os.Open(iconPath)
	if err != nil {
		return "", "", fmt.Errorf("icon %q: %w", iconPath, err)
	}
	defer func() { _ = src.Close() }()

	ext := strings.ToLower(filepath.Ext(iconPath))
	switch ext {
	case ".png", ".svg", ".icns", ".ico", ".xpm":
	case "":
		ext = ".png"
	default:
		return "", "", fmt.Errorf("unsupported icon type %q (want .png, .svg, .icns, .ico, or .xpm)", ext)
	}
	if baseName == "" {
		baseName = "app"
	}
	fileName = baseName + ext
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", "", err
	}
	dest := filepath.Join(destDir, fileName)
	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return "", "", err
	}
	if err := dst.Close(); err != nil {
		return "", "", err
	}
	return baseName, fileName, nil
}

// hicolorIconRel returns the FreeDesktop hicolor theme path for fileName
// (forward slashes). SVG → scalable; otherwise 256x256/apps.
func hicolorIconRel(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	size := "256x256"
	if ext == ".svg" {
		size = "scalable"
	}
	return "usr/share/icons/hicolor/" + size + "/apps/" + fileName
}

// stageFreedesktopIcons stages a root icon, optional AppDir .DirIcon symlink,
// and a hicolor theme copy under destRoot.
func stageFreedesktopIcons(iconPath, destRoot, baseName string, dirIcon bool) (iconKey, fileName string, err error) {
	key, fileName, err := stageIconFile(iconPath, destRoot, baseName)
	if err != nil || fileName == "" {
		return key, fileName, err
	}
	if dirIcon {
		link := filepath.Join(destRoot, ".DirIcon")
		_ = os.Remove(link)
		if err := os.Symlink(fileName, link); err != nil {
			// Fallback copy when symlinks are unavailable.
			src := filepath.Join(destRoot, fileName)
			raw, rerr := os.ReadFile(src)
			if rerr != nil {
				return "", "", fmt.Errorf("DirIcon fallback: %w", rerr)
			}
			if err := os.WriteFile(link, raw, 0o644); err != nil {
				return "", "", err
			}
		}
	}
	hicolor := filepath.Join(destRoot, filepath.FromSlash(hicolorIconRel(fileName)))
	if err := os.MkdirAll(filepath.Dir(hicolor), 0o755); err != nil {
		return "", "", err
	}
	in, err := os.Open(filepath.Join(destRoot, fileName))
	if err != nil {
		return "", "", err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(hicolor, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return "", "", err
	}
	if err := out.Close(); err != nil {
		return "", "", err
	}
	return key, fileName, nil
}
