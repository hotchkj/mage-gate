// Vision: Artifact path helpers: rooted layouts, `..` rejection, and predictable errors on bad inputs.
package harness_test

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	h "github.com/hotchkj/mage-gate/internal/harness"
)

// goosWindows is shared across harness_test files (goconst).
const goosWindows = "windows"

func absPathForResolveTest() string {
	if runtime.GOOS == goosWindows {
		return `C:\abs-out`
	}
	return "/abs-out"
}

func TestResolveWithinRoot_AbsoluteFirstPart(t *testing.T) {
	t.Parallel()
	abs := absPathForResolveTest()
	got, err := h.ResolveWithinRoot("/ignored-root", abs, "extra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(abs, "extra")
	if got != want {
		t.Fatalf("ResolveWithinRoot = %q, want %q", got, want)
	}
}

// Windows drive paths in fsnorm form are not filepath.IsAbs on Unix; first segment must
// still take the absolute branch (same contract as gatecheck path normalization).
func TestResolveWithinRoot_WindowsDriveLexicalOnNonWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == goosWindows {
		t.Skip("native Windows absolute paths are covered by TestResolveWithinRoot_AbsoluteFirstPart")
	}
	got, err := h.ResolveWithinRoot("/ignored-root", `C:\abs-out`, "nested")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Clean(filepath.Join("C:", "abs-out", "nested"))
	if got != want {
		t.Fatalf("ResolveWithinRoot = %q, want %q", got, want)
	}
}

func TestResolveWithinRoot_RelativeUnderRoot(t *testing.T) {
	t.Parallel()
	absRoot, err := filepath.Abs(string(filepath.Separator) + "test-root")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	got, err := h.ResolveWithinRoot("/test-root", "a", "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(absRoot, "a", "b")
	if got != want {
		t.Fatalf("ResolveWithinRoot = %q, want %q", got, want)
	}
}

func TestResolveWithinRoot_RelativeBackslashSegments(t *testing.T) {
	t.Parallel()
	absRoot, err := filepath.Abs(string(filepath.Separator) + "test-root")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	got, err := h.ResolveWithinRoot("/test-root", `a\b`, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(absRoot, "a", "b", "c")
	if got != want {
		t.Fatalf("ResolveWithinRoot = %q, want %q", got, want)
	}
}

func TestResolveWithinRoot_EmptyPathParts(t *testing.T) {
	t.Parallel()
	want, err := filepath.Abs(string(filepath.Separator) + "solo-root")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	got, err := h.ResolveWithinRoot("/solo-root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected root only, got %q, want %q", got, want)
	}
}

func TestResolveWithinRoot_TraversalReturnsError(t *testing.T) {
	t.Parallel()
	_, err := h.ResolveWithinRoot("/root", "../outside")
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
	if !errors.Is(err, h.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestResolveWithinRoot_DotDotInMiddleEscapesRoot(t *testing.T) {
	t.Parallel()
	_, err := h.ResolveWithinRoot("/safe-root", "inside", "..", "..", "etc")
	if err == nil {
		t.Fatal("expected error when cleaned path escapes root")
	}
	if !errors.Is(err, h.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}
