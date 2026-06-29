package specsync

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a minimal SemVer value: just enough to parse, compare, and bump.
// No ranges or constraint solving — hand-rolled to honor the stdlib-only
// invariant. Pre-release and build metadata are preserved on read and dropped on
// a normal bump.
type Version struct {
	Major, Minor, Patch int
	Pre                 string // pre-release, without the leading '-'
	Build               string // build metadata, without the leading '+'
}

// ParseVersion parses MAJOR.MINOR.PATCH[-pre][+build], tolerating a leading "v".
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimSpace(s)
	raw = strings.TrimPrefix(raw, "v")
	var v Version
	if i := strings.IndexByte(raw, '+'); i >= 0 {
		v.Build = raw[i+1:]
		raw = raw[:i]
	}
	if i := strings.IndexByte(raw, '-'); i >= 0 {
		v.Pre = raw[i+1:]
		raw = raw[:i]
	}
	parts := strings.SplitN(raw, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("not a semver: %q", s)
	}
	var err error
	if v.Major, err = atoiStrict(parts[0]); err != nil {
		return Version{}, fmt.Errorf("bad major in %q: %w", s, err)
	}
	if v.Minor, err = atoiStrict(parts[1]); err != nil {
		return Version{}, fmt.Errorf("bad minor in %q: %w", s, err)
	}
	if v.Patch, err = atoiStrict(parts[2]); err != nil {
		return Version{}, fmt.Errorf("bad patch in %q: %w", s, err)
	}
	return v, nil
}

func atoiStrict(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("not a non-negative integer: %q", s)
	}
	return n, nil
}

// String renders the canonical "MAJOR.MINOR.PATCH[-pre][+build]" form.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Bump returns the next version for an impact, dropping pre-release/build
// metadata (a normal release). ImpactNone returns the version unchanged.
func (v Version) Bump(impact ReleaseImpact) Version {
	out := Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	switch impact {
	case ImpactMajor:
		out.Major++
		out.Minor, out.Patch = 0, 0
	case ImpactMinor:
		out.Minor++
		out.Patch = 0
	case ImpactPatch:
		out.Patch++
	default:
		return v
	}
	return out
}
