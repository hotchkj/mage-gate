// Package fsnorm is the single place for lexical path-string normalization at trust boundaries (tool-emitted paths,
// JSON, canonical artifact names, inventories, excludes, [ResolveWithinRoot] fragments after untrusted ingress).
//
// Keep canonical comparisons and merges here ([Canonical], [Rel], [Join], [Base], [Dir]); use OS-native filepath
// only at explicit seams ([ResolveWithinRoot], exec, rooted disk adapters). Cmd subprocess cwd stays native in
// cmdrunner—do not conflate cwd strings with the canonical realm without normalizing first. See
// docs/kb/coding-standards.md: canonicalize before use; never assume filepath.ToSlash alone is enough on Unix.
//
// Canonical does not open files or enforce root containment—pair with harness.ResolveWithinRoot (or equivalent)
// when paths opened on disk must stay under a root.
package fsnorm

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

// ErrRelPathMismatch means target cannot be expressed relative to base in canonical space.
var ErrRelPathMismatch = errors.New("canonical relative path mismatch")

// Canonical returns one forward-slash, filepath.Clean lexical form for comparing or
// matching path-like strings from any GOOS. Backslashes are mapped to '/' first;
// filepath.Clean then filepath.ToSlash so Windows does not reintroduce '\' via Clean.
// Applying Canonical again is a no-op on its output (idempotent); callers may
// canonicalize at both a boundary and an inner layer (e.g. fakes) without changing meaning.
func Canonical(p string) string {
	if p == "" {
		return ""
	}
	slashed := strings.ReplaceAll(p, `\`, `/`)
	return filepath.ToSlash(filepath.Clean(slashed))
}

// Rel returns target relative to base in canonical forward-slash form.
// Inputs are canonical path strings; use OS-native filepath.Rel only at explicit OS seams.
func Rel(base, target string) (string, error) {
	base = canonicalRelDefault(base)
	target = canonicalRelDefault(target)
	baseVol, baseRest := splitCanonicalVolume(base)
	targetVol, targetRest := splitCanonicalVolume(target)
	if baseVol != targetVol || isRooted(baseRest) != isRooted(targetRest) {
		return "", ErrRelPathMismatch
	}
	baseParts := splitCanonicalParts(baseRest)
	targetParts := splitCanonicalParts(targetRest)
	common := commonPrefixLen(baseParts, targetParts)
	relParts := make([]string, 0, len(baseParts)-common+len(targetParts)-common)
	for range baseParts[common:] {
		relParts = append(relParts, "..")
	}
	relParts = append(relParts, targetParts[common:]...)
	if len(relParts) == 0 {
		return ".", nil
	}
	return strings.Join(relParts, "/"), nil
}

// Dir returns the parent directory of p in canonical forward-slash form.
// Input is a canonical path string; use OS-native filepath.Dir only at explicit OS seams.
func Dir(p string) string {
	return Canonical(path.Dir(Canonical(p)))
}

// Join joins path elements and returns canonical forward-slash form.
// Inputs are canonical path strings; use OS-native filepath.Join only at explicit OS seams.
func Join(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	canon := make([]string, len(parts))
	for i, part := range parts {
		canon[i] = Canonical(part)
	}
	return Canonical(path.Join(canon...))
}

// Base returns the final path element of p in canonical forward-slash form.
// Input is a canonical path string; use OS-native filepath.Base only at explicit OS seams.
func Base(p string) string {
	return path.Base(Canonical(p))
}

func canonicalRelDefault(p string) string {
	p = Canonical(p)
	if p == "" {
		return "."
	}
	return p
}

func splitCanonicalVolume(p string) (volume, rest string) {
	if len(p) >= 2 && p[1] == ':' && isASCIIAlpha(p[0]) {
		return strings.ToUpper(p[:2]), p[2:]
	}
	return "", p
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isRooted(p string) bool {
	return strings.HasPrefix(p, "/")
}

func splitCanonicalParts(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "." || p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func commonPrefixLen(left, right []string) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for idx := 0; idx < limit; idx++ {
		if left[idx] != right[idx] {
			return idx
		}
	}
	return limit
}
