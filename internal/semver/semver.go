// Package semver compares release versions well enough for lich's update
// checks: MAJOR.MINOR.PATCH plus the SemVer rules that a pre-release precedes
// the release it qualifies and that two pre-releases order identifier by
// identifier.
package semver

import (
	"strconv"
	"strings"
)

// Less reports whether version a is strictly older than b. Missing or
// non-numeric components sort as 0, a pre-release sorts before the release it
// qualifies (SemVer §11: 1.0.0-rc1 < 1.0.0), and two pre-releases of the same
// version compare identifier by identifier (1.0.0-rc.2 < 1.0.0-rc.10).
func Less(a, b string) bool {
	ca, pa := parse(a)
	cb, pb := parse(b)
	for i := range ca {
		if ca[i] != cb[i] {
			return ca[i] < cb[i]
		}
	}
	switch {
	case pa == pb:
		return false
	case pa == "":
		return false
	case pb == "":
		return true
	}
	return lessPreRelease(strings.Split(pa, "."), strings.Split(pb, "."))
}

// lessPreRelease is SemVer §11.4: numeric identifiers compare as numbers and
// sort before alphanumeric ones, alphanumeric ones compare as text, and a
// shorter list that is a prefix of the longer sorts first.
func lessPreRelease(a, b []string) bool {
	for i := range min(len(a), len(b)) {
		if a[i] == b[i] {
			continue
		}
		na, errA := strconv.Atoi(a[i])
		nb, errB := strconv.Atoi(b[i])
		switch {
		case errA == nil && errB == nil:
			return na < nb
		case errA == nil:
			return true
		case errB == nil:
			return false
		}
		return a[i] < b[i]
	}
	return len(a) < len(b)
}

// IsRelease reports whether v is a MAJOR.MINOR.PATCH release version, with
// no pre-release suffix. A dev build ("dev", or the "-N-gSHA" that git
// describe emits between tags) is not a release, so the update check must not
// treat it as one. Build metadata ("+local.3") is ignored, as it carries no
// precedence (SemVer §10): a build of a release that only labels itself is
// that release, and is offered the next one.
func IsRelease(v string) bool {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if strings.Contains(v, "-") {
		return false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

// IsPreRelease reports whether v is a pre-release lich tags, a release
// version followed by a label and its number (0.93.0-rc.15). The "-N-gSHA"
// that git describe emits between tags is not one.
func IsPreRelease(v string) bool {
	core, pre, ok := strings.Cut(strings.TrimPrefix(v, "v"), "-")
	if !ok || !IsRelease(core) {
		return false
	}
	if i := strings.IndexByte(pre, '+'); i >= 0 {
		pre = pre[:i]
	}
	label, number, ok := strings.Cut(pre, ".")
	if !ok || label == "" || strings.Trim(label, "abcdefghijklmnopqrstuvwxyz") != "" {
		return false
	}
	_, err := strconv.Atoi(number)
	return err == nil
}

// parse splits v into its MAJOR, MINOR and PATCH and its pre-release, "" for
// a release.
func parse(v string) ([3]int, string) {
	v = strings.TrimPrefix(v, "v")
	// Build metadata carries no precedence (SemVer §10), so drop it before the
	// pre-release check — "+" may legally precede nothing else.
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var pre string
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(part)
	}
	return out, pre
}
