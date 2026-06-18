// Package fileopspath provides shared containment helpers for rooted FileOps implementations:
// lexical classification, filepath.Abs/dir/filepath.Rel rooting, and harness-facing walk callbacks.
package fileopspath

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// IsPathEscape reports whether filepath.Rel produced a path that leaves the root
// (for example ".." or "../x").
func IsPathEscape(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalizeParts(pathParts []string) []string {
	out := make([]string, len(pathParts))
	for i := range pathParts {
		out[i] = fsnorm.Canonical(pathParts[i])
	}
	return out
}

// JoinIfFirstSegmentAbsolute returns a cleaned join of pathParts when the first
// segment is already absolute (including Windows volume paths after fsnorm).
func JoinIfFirstSegmentAbsolute(pathParts []string) (string, bool) {
	if len(pathParts) == 0 {
		return "", false
	}
	first := pathParts[0]
	cf := fsnorm.Canonical(first)
	// Match gatecheck / harness path classification for absolute segments: Unix-rooted
	// canonical paths (for example /test-root) are not filepath.IsAbs on Windows but
	// must still take the absolute branch for containment.
	if filepath.IsAbs(first) ||
		filepath.VolumeName(first) != "" ||
		gatecheck.IsWindowsDriveLexicalCanon(cf) ||
		strings.HasPrefix(cf, "/") {
		return filepath.Clean(filepath.Join(pathParts...)), true
	}
	return "", false
}

// LogicalContainedRelative resolves input as a single path segment group (slashes allowed)
// under hostRoot using the same root-relative filepath.Abs + filepath.Rel containment seam as
// harness.ResolveWithinRoot for joined relative segments. It is stricter than ResolveWithinRoot
// for absolute-looking inputs: every resolved path must stay under hostRoot (ResolveWithinRoot’s
// absolute-first path branch is not used for FileOps).
//
// The returned logical path uses forward slashes and matches fsnorm canonical form relative
// to the configured root. It is "." when input resolves exactly to hostRoot's absolute base.
func LogicalContainedRelative(hostRoot, input string) (string, error) {
	part := canonicalizeParts([]string{input})
	absRoot, err := filepath.Abs(filepath.Clean(hostRoot))
	if err != nil {
		return "", fmt.Errorf("resolve fileops root: %w", err)
	}

	var absResolved string
	if joined, ok := JoinIfFirstSegmentAbsolute(part); ok {
		absJoined := filepath.Clean(joined)
		var absJoinErr error
		absResolved, absJoinErr = filepath.Abs(absJoined)
		if absJoinErr != nil {
			return "", fmt.Errorf("resolve confined path %q: %w", absJoined, absJoinErr)
		}
	} else {
		absResolved = filepath.Clean(filepath.Join(append([]string{absRoot}, part...)...))
	}

	rel, err := filepath.Rel(absRoot, absResolved)
	pathDesc := fsnorm.Canonical(input)
	if err != nil {
		return "", fmt.Errorf("%q escapes root %q: %w", pathDesc, fsnorm.Canonical(hostRoot), ErrPathTraversal)
	}
	if IsPathEscape(rel) {
		return "", fmt.Errorf("%q escapes root %q: %w", pathDesc, fsnorm.Canonical(hostRoot), ErrPathTraversal)
	}

	return fsnorm.Canonical(filepath.ToSlash(rel)), nil
}
