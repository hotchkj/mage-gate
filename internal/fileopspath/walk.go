package fileopspath

import (
	"path/filepath"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// DisplayWalkPath rewrites filesystem walk callbacks from an interior rooted location
// to the canonical display root string used by harness comparisons (matching gate root casing).
func DisplayWalkPath(displayRootCanon, interiorCanon, walkFilesystemPath string) string {
	wp := fsnorm.Canonical(filepath.ToSlash(walkFilesystemPath))
	tail, err := fsnorm.Rel(interiorCanon, wp)
	if err != nil {
		return fsnorm.Join(displayRootCanon, wp)
	}
	if tail == "." {
		return displayRootCanon
	}
	return fsnorm.Join(displayRootCanon, tail)
}
