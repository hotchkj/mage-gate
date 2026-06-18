package fileopspath_test

import (
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

func TestDisplayWalkPath_SameInteriorDot(t *testing.T) {
	t.Parallel()
	const display = `/gate/display`
	got := fileopspath.DisplayWalkPath(display, `.`, `.`)
	if got != fsnorm.Canonical(display) {
		t.Fatalf("got %q want %q", got, display)
	}
}

func TestDisplayWalkPath_NormalChildUnderDotInterior(t *testing.T) {
	t.Parallel()
	got := fileopspath.DisplayWalkPath(`/module`, `.`, filepath.Join(`pkg`, `leaf.go`))
	want := `/module/pkg/leaf.go`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDisplayWalkPath_NestedInteriorChild(t *testing.T) {
	t.Parallel()
	// Walk emits paths that include interiorCanon as a prefix (see afero.Walk accumulating filepath.Join(root, name)).
	got := fileopspath.DisplayWalkPath(`/r`, `artifacts`, filepath.Join(`artifacts`, `nested`, `out.go`))
	want := `/r/nested/out.go`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDisplayWalkPath_RelMismatchFallsBackToJoin(t *testing.T) {
	t.Parallel()
	const displayCanon = `C:/display/root`
	const interiorCanon = `C:/inside`
	const stray = `D:/other/volume-path`
	got := fileopspath.DisplayWalkPath(displayCanon, interiorCanon, stray)
	want := fsnorm.Join(displayCanon, fsnorm.Canonical(filepath.ToSlash(stray)))
	if got != want {
		t.Fatalf("got %q want join fallback %q", got, want)
	}
}
