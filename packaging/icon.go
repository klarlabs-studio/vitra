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
