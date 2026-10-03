//go:build windows

package windows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"go.klarlabs.de/vitra/domain"
)

// runKey is the per-user key whose values Windows starts at login.
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

var (
	advapi32          = syscall.NewLazyDLL("advapi32.dll")
	procRegSetValueEx = advapi32.NewProc("RegSetValueExW")
	procRegCreateKey  = advapi32.NewProc("RegCreateKeyExW")
	procRegDeleteVal  = advapi32.NewProc("RegDeleteValueW")
)

// errNoRunKey is returned by openRunKey when the Run key does not exist,
// which it does not on a fresh profile until something starts at login.
var errNoRunKey = errors.New("no Run key")

// openRunKey opens HKCU\...\Run with access.
func openRunKey(access uint32) (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(runKey)
	if err != nil {
		return 0, err
	}
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, name, 0, access, &key); err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return 0, errNoRunKey
		}
		return 0, fmt.Errorf("open %s: %w", runKey, err)
	}
	return key, nil
}

// createRunKey opens HKCU\...\Run for writing, creating it if needed.
func createRunKey() (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(runKey)
	if err != nil {
		return 0, err
	}
	var key syscall.Handle
	r, _, _ := procRegCreateKey.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(name)), 0, 0, 0,
		uintptr(syscall.KEY_SET_VALUE), 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return 0, fmt.Errorf("create %s: %w", runKey, syscall.Errno(r))
	}
	return key, nil
}

func runValueName(appID string) (*uint16, error) {
	if appID == "" {
		return nil, &domain.ErrValidation{Message: "app id required for a login item"}
	}
	return syscall.UTF16PtrFromString(safeName(appID))
}

// LoginItemEnabled reports whether the app has a value under the Run key.
func (h *Host) LoginItemEnabled(appID string) (bool, error) {
	name, err := runValueName(appID)
	if err != nil {
		return false, err
	}
	key, err := openRunKey(syscall.KEY_QUERY_VALUE)
	if errors.Is(err, errNoRunKey) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = syscall.RegCloseKey(key) }()
	var typ, size uint32
	err = syscall.RegQueryValueEx(key, name, nil, &typ, nil, &size)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return false, nil
	}
	return err == nil, err
}

// SetLoginItem sets (or deletes) the app's value under the Run key, so
// Windows starts execPath, quoted, when the user logs in. It does not need
// the WebView host.
func (h *Host) SetLoginItem(appID, execPath string, enabled bool) error {
	name, err := runValueName(appID)
	if err != nil {
		return err
	}
	if !enabled {
		key, err := openRunKey(syscall.KEY_SET_VALUE)
		if errors.Is(err, errNoRunKey) {
			return nil
		}
		if err != nil {
			return err
		}
		defer func() { _ = syscall.RegCloseKey(key) }()
		r, _, _ := procRegDeleteVal.Call(uintptr(key), uintptr(unsafe.Pointer(name)))
		if r != 0 && syscall.Errno(r) != syscall.ERROR_FILE_NOT_FOUND {
			return fmt.Errorf("delete Run value: %w", syscall.Errno(r))
		}
		return nil
	}
	if execPath == "" {
		return &domain.ErrValidation{Message: "executable path required for a login item"}
	}
	abs, err := filepath.Abs(execPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	data, err := syscall.UTF16FromString(`"` + abs + `"`)
	if err != nil {
		return err
	}
	key, err := createRunKey()
	if err != nil {
		return err
	}
	defer func() { _ = syscall.RegCloseKey(key) }()
	r, _, _ := procRegSetValueEx.Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0, uintptr(syscall.REG_SZ),
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if r != 0 {
		return fmt.Errorf("set Run value: %w", syscall.Errno(r))
	}
	return nil
}

// runValue reads the app's Run value (tests).
func runValue(appID string) (string, error) {
	name, err := runValueName(appID)
	if err != nil {
		return "", err
	}
	key, err := openRunKey(syscall.KEY_QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer func() { _ = syscall.RegCloseKey(key) }()
	buf := make([]uint16, syscall.MAX_PATH*2)
	size := uint32(len(buf) * 2)
	var typ uint32
	if err := syscall.RegQueryValueEx(key, name, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &size); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buf), nil
}
