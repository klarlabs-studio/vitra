package darwin

import (
	"bytes"
	"fmt"
	"os/exec"
)

// clipboardGetRunner reads pasteboard text. Tests replace this.
var clipboardGetRunner = func() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// clipboardSetRunner writes pasteboard text. Tests replace this.
var clipboardSetRunner = func(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

// ClipboardGet returns the macOS pasteboard text via pbpaste.
// Does not require the WKWebView host.
func (h *Host) ClipboardGet() (string, error) {
	text, err := clipboardGetRunner()
	if err != nil {
		return "", fmt.Errorf("pbpaste: %w", err)
	}
	return text, nil
}

// ClipboardSet writes text to the macOS pasteboard via pbcopy.
// Does not require the WKWebView host.
func (h *Host) ClipboardSet(text string) error {
	if err := clipboardSetRunner(text); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}
