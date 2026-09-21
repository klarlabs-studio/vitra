package windows

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// openURLRunner starts the platform opener. Tests replace this to capture argv.
var openURLRunner = func(ctx context.Context, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	return cmd.Start()
}

// OpenURL launches the system default handler for an http(s) or mailto URL via
// `cmd /c start`. Does not require the native WebView2 host.
func (h *Host) OpenURL(ctx context.Context, rawURL string) error {
	u, err := parseOpenURL(rawURL)
	if err != nil {
		return err
	}
	// Empty title argument after start avoids treating the URL as a window title.
	if err := openURLRunner(ctx, "cmd", "/c", "start", "", u); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func parseOpenURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
	default:
		return "", fmt.Errorf("unsupported url scheme %q (allowed: http, https, mailto)", u.Scheme)
	}
	if u.Scheme != "mailto" && u.Host == "" {
		return "", fmt.Errorf("url host is required")
	}
	return u.String(), nil
}
