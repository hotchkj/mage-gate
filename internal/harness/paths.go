// Vision: Artifact directory layout under a gate root—canonical paths and hard blocks on traversal (`..`).
package harness

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

func canonicalizePathParts(pathParts []string) []string {
	out := make([]string, len(pathParts))
	for i, p := range pathParts {
		out[i] = fsnorm.Canonical(p)
	}
	return out
}

// ResolveWithinRoot projects pathParts onto a resolved path under root for host/OS boundaries—containment
// enforced with filepath.Abs(Rel) semantics—ErrPathTraversal on escape (for example ../outside relative to root).
//
// Lexical fragments are canonicalized before joining; ResolveWithinRoot is not the default producer of
// paths passed to FileOps for ordinary artifacts—prefer canonical logical paths ([internal/fsnorm],
// artifact projections) for in-root IO; reserve this helper for explicit host containment (disk, native argv,
// tooling that requires an OS path).
//
// The first-segment-absolute branch exists because hosts return absolute dirs from temp creation: an artifact subtree
// may already be absolute and must flow through untouched rather than joined under root.
func ResolveWithinRoot(root string, pathParts ...string) (string, error) {
	root = fsnorm.Canonical(root)
	pathParts = canonicalizePathParts(pathParts)

	if joined, ok := fileopspath.JoinIfFirstSegmentAbsolute(pathParts); ok {
		return joined, nil
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	resolved := filepath.Clean(filepath.Join(append([]string{absRoot}, pathParts...)...))
	rel, err := filepath.Rel(absRoot, resolved)
	pathDesc := strings.Join(pathParts, "/")
	if err != nil {
		return "", fmt.Errorf("%w: %q escapes root %q", ErrPathTraversal, pathDesc, root)
	}
	if fileopspath.IsPathEscape(rel) {
		return "", fmt.Errorf("%w: %q escapes root %q", ErrPathTraversal, pathDesc, root)
	}
	return resolved, nil
}
