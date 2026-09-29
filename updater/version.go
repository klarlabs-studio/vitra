package updater

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareVersions compares two SemVer 2.0.0 versions by precedence and
// returns -1, 0, or +1. A leading "v" is accepted; build metadata ("+...")
// is ignored, as the spec requires.
func CompareVersions(a, b string) (int, error) {
	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range va.core {
		if c := compareInt(va.core[i], vb.core[i]); c != 0 {
			return c, nil
		}
	}
	return comparePrerelease(va.pre, vb.pre), nil
}

type version struct {
	core [3]uint64
	pre  []string
}

func parseVersion(s string) (version, error) {
	raw := s
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var v version
	core := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core = s[:i]
		v.pre = strings.Split(s[i+1:], ".")
		for _, id := range v.pre {
			if id == "" || (isNumeric(id) && len(id) > 1 && id[0] == '0') {
				return version{}, fmt.Errorf("invalid version %q: bad prerelease identifier %q", raw, id)
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("invalid version %q: want MAJOR.MINOR.PATCH", raw)
	}
	for i, p := range parts {
		if !isNumeric(p) || (len(p) > 1 && p[0] == '0') {
			return version{}, fmt.Errorf("invalid version %q: bad number %q", raw, p)
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("invalid version %q: %w", raw, err)
		}
		v.core[i] = n
	}
	return v, nil
}

// comparePrerelease applies SemVer §11.3–11.4: a release outranks any
// prerelease; identifiers compare numerically when both are numeric,
// otherwise lexically, with numeric below alphanumeric; a longer list wins
// when all shared identifiers are equal.
func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		an, bn := isNumeric(a[i]), isNumeric(b[i])
		switch {
		case an && bn:
			x, _ := strconv.ParseUint(a[i], 10, 64)
			y, _ := strconv.ParseUint(b[i], 10, 64)
			if c := compareInt(x, y); c != 0 {
				return c
			}
		case an:
			return -1
		case bn:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return compareInt(uint64(len(a)), uint64(len(b)))
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func compareInt(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
