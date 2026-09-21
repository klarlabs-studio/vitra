//go:build windows

package windows

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func runtimeDir() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "vitra-locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func lockPath(appID string) (string, error) {
	dir, err := runtimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appID+".lock"), nil
}

func sockPath(appID string) (string, error) {
	dir, err := runtimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appID+".sock"), nil
}

// TrySingleInstance acquires an exclusive holder file under LOCALAPPDATA.
// held is false when another instance already owns the lock.
func (h *Host) TrySingleInstance(appID string) (held bool, release func(), err error) {
	if appID == "" {
		return false, nil, fmt.Errorf("app id required for single-instance lock")
	}
	path, err := lockPath(appID)
	if err != nil {
		return false, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return false, func() {}, nil
		}
		return false, nil, err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	release = func() {
		_ = f.Close()
		_ = os.Remove(path)
	}
	return true, release, nil
}

// DeepLinksFromArgs returns argv entries that look like absolute URLs with a scheme.
func DeepLinksFromArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if i := strings.Index(a, "://"); i > 0 {
			scheme := a[:i]
			ok := true
			for _, r := range scheme {
				if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '+' && r != '-' && r != '.' {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, a)
			}
		}
	}
	return out
}

// StartDeepLinkBridge listens on a per-app unix socket (supported on modern
// Windows) and invokes onLink for each newline-delimited URL.
func (h *Host) StartDeepLinkBridge(appID string, onLink func(raw string)) (stop func(), err error) {
	if appID == "" {
		return nil, fmt.Errorf("app id required for deep-link bridge")
	}
	if onLink == nil {
		return nil, fmt.Errorf("deep-link handler is required")
	}
	path, err := sockPath(appID)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				sc := bufio.NewScanner(c)
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					if line != "" {
						onLink(line)
					}
				}
			}(conn)
		}
	}()

	stop = func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
		_ = os.Remove(path)
	}
	return stop, nil
}

// ForwardToPrimary sends URLs to the primary instance deep-link socket.
func (h *Host) ForwardToPrimary(appID string, urls []string) (ok bool, err error) {
	if appID == "" {
		return false, fmt.Errorf("app id required for deep-link forward")
	}
	if len(urls) == 0 {
		return false, nil
	}
	path, err := sockPath(appID)
	if err != nil {
		return false, err
	}
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return false, nil
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	for _, u := range urls {
		if _, err := fmt.Fprintln(conn, u); err != nil {
			return false, err
		}
	}
	return true, nil
}
