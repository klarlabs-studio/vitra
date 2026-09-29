package domain_test

import (
	"errors"
	"path"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func projectScope() domain.PathScope {
	return domain.PathScope{
		Allow: []string{"/project/**"},
		Deny:  []string{"/project/.secrets/**", "/project/**/*.pem"},
	}
}

// Each case is a known bypass of a naive segment matcher. None may be allowed.
func TestPathScope_RejectsBypasses(t *testing.T) {
	scope := projectScope()
	for name, candidate := range map[string]string{
		"deny is case-insensitive (APFS/NTFS)":  "/project/.SECRETS/key",
		"deny is case-insensitive in pattern":   "/project/.Secrets/key",
		"partial-segment glob in deny":          "/project/server.pem",
		"nested partial-segment glob in deny":   "/project/certs/tls/server.pem",
		"partial-segment glob case-insensitive": "/project/server.PEM",
		"backslash traversal":                   `/project/..\..\etc\passwd`,
		"mixed separator traversal":             `/project/a\../../etc/passwd`,
		"backslash into denied dir":             `/project\.secrets\key`,
		"relative path":                         "project/relative.txt",
		"dot-relative path":                     "./project/relative.txt",
		"UNC path":                              `\\evil\share\project\x`,
		"forward-slash UNC path":                "//evil/share/project/x",
		"win32 device namespace":                `\\?\C:\project\x`,
		"drive-relative path":                   "C:project/x",
	} {
		t.Run(name, func(t *testing.T) {
			if ok, code, reason := scope.Matches(candidate); ok {
				t.Fatalf("%q allowed (code=%s reason=%s)", candidate, code, reason)
			}
		})
	}
}

func TestPathScope_AllowsLegitimatePaths(t *testing.T) {
	scope := projectScope()
	for _, candidate := range []string{
		"/project/src/main.go",
		"/project/certs/readme.md",
		"/project/.secretsauce/notes", // prefix of a denied name, but a different segment
	} {
		if ok, code, reason := scope.Matches(candidate); !ok {
			t.Fatalf("%q denied: %s %s", candidate, code, reason)
		}
	}
}

func TestPathScope_WindowsDrivePaths(t *testing.T) {
	scope := domain.PathScope{
		Allow: []string{`C:\Users\dev\project\**`},
		Deny:  []string{`C:\Users\dev\project\.secrets\**`},
	}
	if ok, code, reason := scope.Matches(`C:\Users\dev\project\src\main.go`); !ok {
		t.Fatalf("drive path denied: %s %s", code, reason)
	}
	if ok, _, _ := scope.Matches("C:/Users/dev/project/src/main.go"); !ok {
		t.Fatal("forward-slash drive path denied")
	}
	for _, bad := range []string{
		`c:\users\dev\project\.SECRETS\key`,
		`C:\Users\dev\project\..\..\Windows\System32\config\SAM`,
	} {
		if ok, _, _ := scope.Matches(bad); ok {
			t.Fatalf("%q allowed", bad)
		}
	}
}

func TestNewCapabilityGrant_RejectsMalformedPathPatterns(t *testing.T) {
	for _, scope := range []domain.PathScope{
		{Allow: []string{"/project/[a-"}},
		{Allow: []string{"/project/**"}, Deny: []string{"/project/[z"}},
		{Allow: []string{"relative/**"}},
	} {
		_, err := domain.NewCapabilityGrant("g", "d",
			[]domain.WindowID{"main"},
			[]domain.Origin{domain.OriginPackagedLocal},
			[]domain.PermissionSpec{{Name: "fs.read", PathScope: &scope}},
		)
		if !errors.Is(err, &domain.ErrValidation{}) {
			t.Fatalf("scope %+v: expected validation error, got %v", scope, err)
		}
	}
}

// FuzzPathScope_Matches checks the scope's security properties for arbitrary
// input: anything allowed is absolute, traversal-free, inside the allowed
// root, and outside the denied subtree under any letter case.
func FuzzPathScope_Matches(f *testing.F) {
	for _, seed := range []string{
		"/project/src/main.go",
		"/project/.secrets/key",
		"/project/.SECRETS/key",
		`/project/..\..\etc\passwd`,
		"project/x",
		`\\evil\share`,
		"/project/a/b/c.pem",
		"/project//./x",
		"C:/project/x",
		"/project/\x00",
	} {
		f.Add(seed)
	}
	scope := projectScope()
	f.Fuzz(func(t *testing.T, candidate string) {
		ok, code, _ := scope.Matches(candidate)
		if !ok {
			if code != domain.DenialPathDenied && code != domain.DenialPathOutOfScope {
				t.Fatalf("%q: unexpected denial code %q", candidate, code)
			}
			return
		}
		norm := strings.ReplaceAll(strings.TrimSpace(candidate), `\`, "/")
		for _, seg := range strings.Split(norm, "/") {
			if seg == ".." {
				t.Fatalf("%q allowed with traversal", candidate)
			}
		}
		clean := path.Clean(norm)
		lower := strings.ToLower(clean)
		// "/project/**" deliberately covers the root directory itself.
		if clean != "/project" && !strings.HasPrefix(clean, "/project/") {
			t.Fatalf("%q allowed outside /project/", candidate)
		}
		if lower == "/project/.secrets" || strings.HasPrefix(lower, "/project/.secrets/") {
			t.Fatalf("%q allowed inside denied subtree", candidate)
		}
		if strings.HasSuffix(lower, ".pem") {
			t.Fatalf("%q allowed despite *.pem deny", candidate)
		}
	})
}
