package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// scaffoldGrant generates a starter app with the given extra `vitra new`
// arguments and returns its main.go without comment lines.
func scaffoldGrant(t *testing.T, args ...string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	capture(t, func() {
		if err := run(append([]string{"new", dir}, args...)); err != nil {
			t.Fatal(err)
		}
	})
	src, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	var active []string
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			active = append(active, line)
		}
	}
	return strings.Join(active, "\n")
}

// The starter is what most apps are built from, so its defaults are the
// project's real security posture.
func TestScaffold_GrantsLeastPrivilegeByDefault(t *testing.T) {
	src := scaffoldGrant(t)

	for _, perm := range []string{
		"desktop.PermFSRead",
		"desktop.PermFSWrite",
		"desktop.PermPathOpen",
		"desktop.PermOpenURL",
		"desktop.PermClipboardRead",
	} {
		if regexp.MustCompile(`\{Name:\s*` + regexp.QuoteMeta(perm) + `\b`).MatchString(src) {
			t.Errorf("scaffold grants %s by default", perm)
		}
	}
	// A secondary window must not inherit the primary window's authority.
	if regexp.MustCompile(`WindowID\{[^}]*"aux"[^}]*\}[^)]*desktop\.Perm`).MatchString(src) {
		t.Error("scaffold grants desktop permissions to the aux window")
	}
	if strings.Contains(src, "grant, _ :=") || strings.Contains(src, "_ = rt.RegisterGrant") {
		t.Error("scaffold ignores grant errors")
	}
}
