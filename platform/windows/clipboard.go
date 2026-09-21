package windows

import (
	"bytes"
	"fmt"
	"os/exec"
)

// clipboardGetRunner reads clipboard text. Tests replace this.
var clipboardGetRunner = func() (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// clipboardSetRunner writes clipboard text. Tests replace this.
var clipboardSetRunner = func(text string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Value $input")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

// ClipboardGet returns clipboard text via PowerShell Get-Clipboard.
// Does not require the native WebView2 host.
func (h *Host) ClipboardGet() (string, error) {
	text, err := clipboardGetRunner()
	if err != nil {
		return "", fmt.Errorf("clipboard get: %w", err)
	}
	return text, nil
}

// ClipboardSet writes clipboard text via PowerShell Set-Clipboard.
// Does not require the native WebView2 host.
func (h *Host) ClipboardSet(text string) error {
	if err := clipboardSetRunner(text); err != nil {
		return fmt.Errorf("clipboard set: %w", err)
	}
	return nil
}
