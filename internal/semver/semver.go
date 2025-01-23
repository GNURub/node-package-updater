package semver

import (
	"fmt"
	"sort"
	"strings"
)

var prefixes = []string{">=", ">", "^", "~"}

type Version struct {
	prefix     string
	major      string
	minor      string
	patch      string
	short      string
	prerelease string
	build      string
	version    string
	isValid    bool
}

func NewSemver(version string) *Version {
	pv, isValid := parse(version)
	pv.isValid = isValid
	return &pv
}

func (v *Version) IsValid() bool {
	return v.isValid
}

func (v *Version) Canonical() string {
	if !v.isValid {
		return ""
	}
	if v.build != "" {
		return v.version[:len(v.version)-len(v.build)]
	}
	if v.short != "" {
		return v.version + v.short
	}
	return v.version
}

func (v *Version) MajorMinor() string {
	if !v.isValid {
		return ""
	}
	i := 1 + len(v.major)
	if j := i + 1 + len(v.minor); j <= len(v.version) && v.version[i] == '.' && v.version[i+1:j] == v.minor {
		return v.version[:j]
	}
	return v.version[:i] + "." + v.minor
}

// Prerelease returns the prerelease suffix of the semantic version v.
// For example, Prerelease("v2.1.0-pre+meta") == "-pre".
// If v is an invalid semantic version string, Prerelease returns the empty string.
func (v *Version) Prerelease() string {
	if !v.isValid {
		return ""
	}
	return v.prerelease
}

// Build returns the build suffix of the semantic version v.
// For example, Build("v2.1.0+meta") == "+meta".
// If v is an invalid semantic version string, Build returns the empty string.
func (v *Version) Build() string {
	if !v.isValid {
		return ""
	}
	return v.build
}

// Compare returns an integer comparing two versions according to
// semantic version precedence.
// The result will be 0 if v == w, -1 if v < w, or +1 if v > w.
//
// An invalid semantic version string is considered less than a valid one.
// All invalid semantic version strings compare equal to each other.
func (v *Version) Compare(w *Version) int {
	if !v.isValid && !w.isValid {
		return 0
	}
	if !v.isValid {
		return -1
	}
	if !w.isValid {
		return +1
	}
	if c := compareInt(v.major, w.major); c != 0 {
		return c
	}
	if c := compareInt(v.minor, w.minor); c != 0 {
		return c
	}
	if c := compareInt(v.patch, w.patch); c != 0 {
		return c
	}
	return comparePrerelease(v.prerelease, w.prerelease)
}

// ByVersion implements [sort.Interface] for sorting semantic version strings.
type ByVersion []*Version

func (vs ByVersion) Len() int      { return len(vs) }
func (vs ByVersion) Swap(i, j int) { vs[i], vs[j] = vs[j], vs[i] }
func (vs ByVersion) Less(i, j int) bool {
	cmp := vs[i].Compare(vs[j])
	if cmp != 0 {
		return cmp < 0
	}
	return vs[i].version < vs[j].version
}

func (v *Version) String() string {
	return v.version
}

func (v *Version) StringWithPrefix() string {
	return fmt.Sprintf("%s%s", v.prefix, v.version)
}

func (v *Version) GetMatchPatchVersion(vs ByVersion) *Version {
	// Sort the versions in descending order
	sort.Sort(sort.Reverse(vs))

	for _, candidate := range vs {
		if v.major == candidate.major && v.minor == candidate.minor && candidate.prerelease == "" {
			if v.prefix == "" || v.prefix == candidate.prefix {
				return candidate
			}
		}
	}

	return nil
}

func (v *Version) GetMatchMinorVersion(vs ByVersion) *Version {
	// Sort the versions in descending order
	sort.Sort(sort.Reverse(vs))

	for _, candidate := range vs {
		if v.major == candidate.major && candidate.prerelease == "" {
			if v.prefix == "" || v.prefix == candidate.prefix {
				return candidate
			}
		}
	}

	return nil
}

func (v *Version) GetMatchLatestVersion(vs ByVersion) *Version {
	sort.Sort(sort.Reverse(vs))

	for _, candidate := range vs {
		if v.prefix == "" || v.prefix == candidate.prefix && candidate.prerelease == "" {
			return candidate
		}
	}

	return nil
}

// Sort sorts a list of semantic version strings using [ByVersion].
func Sort(list []*Version) {
	sort.Sort(ByVersion(list))
}

func parse(v string) (p Version, ok bool) {
	if v == "" {
		return
	}
	if v[0] == 'v' {
		v = v[1:]
	}

	p.version = v

	prefix := getPrefix(v)
	if prefix != "" {
		p.prefix = prefix
		v = v[len(prefix):]
	}

	p.major, v, ok = parseInt(v[1:])
	if !ok {
		return
	}
	if v == "" {
		p.minor = "0"
		p.patch = "0"
		p.short = ".0.0"
		return
	}
	if v[0] != '.' {
		ok = false
		return
	}
	p.minor, v, ok = parseInt(v[1:])
	if !ok {
		return
	}
	if v == "" {
		p.patch = "0"
		p.short = ".0"
		return
	}
	if v[0] != '.' {
		ok = false
		return
	}
	p.patch, v, ok = parseInt(v[1:])
	if !ok {
		return
	}
	if len(v) > 0 && v[0] == '-' {
		p.prerelease, v, ok = parsePrerelease(v)
		if !ok {
			return
		}
	}
	if len(v) > 0 && v[0] == '+' {
		p.build, v, ok = parseBuild(v)
		if !ok {
			return
		}
	}
	if v != "" {
		ok = false
		return
	}
	ok = true
	return
}

func parseInt(v string) (t, rest string, ok bool) {
	if v == "" {
		return
	}
	if v[0] < '0' || '9' < v[0] {
		return
	}
	i := 1
	for i < len(v) && '0' <= v[i] && v[i] <= '9' {
		i++
	}
	if v[0] == '0' && i != 1 {
		return
	}
	return v[:i], v[i:], true
}

func parsePrerelease(v string) (t, rest string, ok bool) {
	// "A pre-release version MAY be denoted by appending a hyphen and
	// a series of dot separated identifiers immediately following the patch version.
	// Identifiers MUST comprise only ASCII alphanumerics and hyphen [0-9A-Za-z-].
	// Identifiers MUST NOT be empty. Numeric identifiers MUST NOT include leading zeroes."
	if v == "" || v[0] != '-' {
		return
	}
	i := 1
	start := 1
	for i < len(v) && v[i] != '+' {
		if !isIdentChar(v[i]) && v[i] != '.' {
			return
		}
		if v[i] == '.' {
			if start == i || isBadNum(v[start:i]) {
				return
			}
			start = i + 1
		}
		i++
	}
	if start == i || isBadNum(v[start:i]) {
		return
	}
	return v[:i], v[i:], true
}

func parseBuild(v string) (t, rest string, ok bool) {
	if v == "" || v[0] != '+' {
		return
	}
	i := 1
	start := 1
	for i < len(v) {
		if !isIdentChar(v[i]) && v[i] != '.' {
			return
		}
		if v[i] == '.' {
			if start == i {
				return
			}
			start = i + 1
		}
		i++
	}
	if start == i {
		return
	}
	return v[:i], v[i:], true
}

func isIdentChar(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-'
}

func isBadNum(v string) bool {
	i := 0
	for i < len(v) && '0' <= v[i] && v[i] <= '9' {
		i++
	}
	return i == len(v) && i > 1 && v[0] == '0'
}

func isNum(v string) bool {
	i := 0
	for i < len(v) && '0' <= v[i] && v[i] <= '9' {
		i++
	}
	return i == len(v)
}

func compareInt(x, y string) int {
	if x == y {
		return 0
	}
	if len(x) < len(y) {
		return -1
	}
	if len(x) > len(y) {
		return +1
	}
	if x < y {
		return -1
	} else {
		return +1
	}
}

func comparePrerelease(x, y string) int {
	// "When major, minor, and patch are equal, a pre-release version has
	// lower precedence than a normal version.
	// Example: 1.0.0-alpha < 1.0.0.
	// Precedence for two pre-release versions with the same major, minor,
	// and patch version MUST be determined by comparing each dot separated
	// identifier from left to right until a difference is found as follows:
	// identifiers consisting of only digits are compared numerically and
	// identifiers with letters or hyphens are compared lexically in ASCII
	// sort order. Numeric identifiers always have lower precedence than
	// non-numeric identifiers. A larger set of pre-release fields has a
	// higher precedence than a smaller set, if all of the preceding
	// identifiers are equal.
	// Example: 1.0.0-alpha < 1.0.0-alpha.1 < 1.0.0-alpha.beta <
	// 1.0.0-beta < 1.0.0-beta.2 < 1.0.0-beta.11 < 1.0.0-rc.1 < 1.0.0."
	if x == y {
		return 0
	}
	if x == "" {
		return +1
	}
	if y == "" {
		return -1
	}
	for x != "" && y != "" {
		x = x[1:] // skip - or .
		y = y[1:] // skip - or .
		var dx, dy string
		dx, x = nextIdent(x)
		dy, y = nextIdent(y)
		if dx != dy {
			ix := isNum(dx)
			iy := isNum(dy)
			if ix != iy {
				if ix {
					return -1
				} else {
					return +1
				}
			}
			if ix {
				if len(dx) < len(dy) {
					return -1
				}
				if len(dx) > len(dy) {
					return +1
				}
			}
			if dx < dy {
				return -1
			} else {
				return +1
			}
		}
	}
	if x == "" {
		return -1
	} else {
		return +1
	}
}

func nextIdent(x string) (dx, rest string) {
	i := 0
	for i < len(x) && x[i] != '.' {
		i++
	}
	return x[:i], x[i:]
}

func getPrefix(version string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(version, prefix) {
			return prefix
		}
	}

	return ""
}
