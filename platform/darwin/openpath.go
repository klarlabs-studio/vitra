package darwin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// openPathRunner starts the platform path opener. Tests replace this to capture argv.
var openPathRunner = func(ctx context.Context, bin string, args ...string) error {
	return openURLRunner(ctx, bin, args...)
}

// OpenPath launches the system default handler for an absolute local path via
// macOS `open`. Does not require the WKWebView host.
func (h *Host) OpenPath(ctx context.Context, path string) error {
	path, err := parseOpenPath(path)
	if err != nil {
		return err
	}
	if err := openPathRunner(ctx, "open", path); err != nil {
		return fmt.Errorf("open: %w", err)
	}
	return nil
}

func parseOpenPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	lower := strings.ToLower(path)
	if strings.Contains(path, "://") || strings.HasPrefix(lower, "file:") {
		return "", fmt.Errorf("path must be a local filesystem path, not a URL")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute")
	}
	return filepath.Clean(path), nil
}
