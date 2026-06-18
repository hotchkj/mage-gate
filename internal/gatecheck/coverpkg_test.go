// Vision: coverpkg list construction: quality-scope packages, excludes, and test-file pattern stripping.
package gatecheck

import (
	"slices"
	"testing"
)

func TestFilterCoverpkg_NoExclusions(t *testing.T) {
	t.Parallel()
	paths := []string{"github.com/example/pkg", "github.com/example/internal"}
	result := FilterCoverpkg(paths, nil)
	want := []string{"github.com/example/pkg", "github.com/example/internal"}
	if !slices.Equal(result, want) {
		t.Fatalf("result = %v, want %v", result, want)
	}
}

func TestFilterCoverpkg_WithExclusions(t *testing.T) {
	t.Parallel()
	paths := []string{
		"github.com/example/pkg",
		"github.com/example/internal/gatetest",
		"github.com/example/tools/gatekeepers",
	}
	exclude := []string{"internal/gatetest", "tools/gatekeepers"}
	result := FilterCoverpkg(paths, exclude)
	want := []string{"github.com/example/pkg"}
	if !slices.Equal(result, want) {
		t.Fatalf("result = %v, want %v", result, want)
	}
}

func TestFilterCoverpkg_ExactMatch(t *testing.T) {
	t.Parallel()
	paths := []string{"tools/gatekeepers"}
	exclude := []string{"tools/gatekeepers"}
	result := FilterCoverpkg(paths, exclude)
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

// BDD and harness import paths (features/, gatetest) must be droppable from coverpkg
// so gate-reported % is not a naive merge total diluted by 0% harness packages.
func TestFilterCoverpkg_ExcludesBDDAndHarnessPathSegments(t *testing.T) {
	t.Parallel()
	paths := []string{
		"github.com/hotchkj/mage-gate/gate",
		"github.com/hotchkj/mage-gate/features/internal/steps",
		"github.com/hotchkj/mage-gate/gatetest",
	}
	exclude := []string{"features", "gatetest", "integration", "testdata"}
	result := FilterCoverpkg(paths, exclude)
	want := []string{"github.com/hotchkj/mage-gate/gate"}
	if !slices.Equal(result, want) {
		t.Fatalf("result = %v, want %v", result, want)
	}
}

func TestParseExcludeSegments(t *testing.T) {
	t.Parallel()
	segments := ParseExcludeSegments("internal/gatetest,tools/gatekeepers")
	if len(segments) != 2 {
		t.Fatalf("expected 2, got %d", len(segments))
	}
	if segments[0] != "internal/gatetest" {
		t.Fatalf("expected internal/gatetest, got %s", segments[0])
	}
}

func TestParseExcludeSegments_Empty(t *testing.T) {
	t.Parallel()
	segments := ParseExcludeSegments("")
	if len(segments) != 0 {
		t.Fatalf("expected 0, got %d", len(segments))
	}
}

func TestParseExcludeSegments_NormalizesWindowsDelimiters(t *testing.T) {
	t.Parallel()
	segments := ParseExcludeSegments(`\vendor\ , internal\testutil`)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %v", segments)
	}
	if segments[0] != "vendor" {
		t.Fatalf("segment[0] = %q, want vendor", segments[0])
	}
	if segments[1] != "internal/testutil" {
		t.Fatalf("segment[1] = %q, want internal/testutil", segments[1])
	}
}

func TestContainsSegment_WindowsAbsoluteNested(t *testing.T) {
	t.Parallel()
	path := `D:\Git\mage-gate\testdata\failures`
	if !containsSegment(path, "testdata") {
		t.Fatalf("containsSegment(%q, %q) = false, want true", path, "testdata")
	}
}

func TestContainsSegment_WindowsMixedSeparators(t *testing.T) {
	t.Parallel()
	path := `D:\Git/mage-gate\testdata`
	if !containsSegment(path, "testdata") {
		t.Fatalf("containsSegment(%q, %q) = false, want true", path, "testdata")
	}
}

func TestContainsSegment_ForwardSlashImportPath(t *testing.T) {
	t.Parallel()
	path := "github.com/foo/testdata"
	if !containsSegment(path, "testdata") {
		t.Fatalf("containsSegment(%q, %q) = false, want true", path, "testdata")
	}
}

func TestContainsSegment_NoFalsePrefixMatch(t *testing.T) {
	t.Parallel()
	path := `D:\Git\mage-gate\testdataplus`
	if containsSegment(path, "testdata") {
		t.Fatalf("containsSegment(%q, %q) = true, want false", path, "testdata")
	}
}

func TestContainsSegment_ExactMatch(t *testing.T) {
	t.Parallel()
	if !containsSegment("testdata", "testdata") {
		t.Fatal(`containsSegment("testdata", "testdata") = false, want true`)
	}
}

func TestContainsSegment_SegmentBackslashOnlyLexical(t *testing.T) {
	t.Parallel()
	if !containsSegment("github.com/example/internal/gatetest", `internal\gatetest`) {
		t.Fatal("segment with backslashes should match after lexical slash norm only")
	}
}

// Segment must not be filepath.Clean'd: "foo/../bar" is a literal needle, not "bar".
func TestContainsSegment_SegmentDotDotNotCollapsed(t *testing.T) {
	t.Parallel()
	path := "github.com/example/foo/bar"
	if containsSegment(path, "foo/../bar") {
		t.Fatal("segment must not be Clean'd: foo/../bar must not match as if it were foo/bar")
	}
}

func TestContainsSegment_TrailingSegmentSuffix(t *testing.T) {
	t.Parallel()
	if !containsSegment("github.com/example/integration", "integration") {
		t.Fatal("expected suffix match via /+segment")
	}
}

func TestContainsSegment_MiddleSegment(t *testing.T) {
	t.Parallel()
	if !containsSegment("github.com/example/pkg/integration/test", "integration") {
		t.Fatal("expected middle segment match")
	}
}

func TestFilterTestDurations_NoExclusions(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "pkg/a", Test: "TestA", Elapsed: 1.0},
		{Package: "pkg/b", Test: "TestB", Elapsed: 2.0},
	}
	result := FilterTestDurations(tests, nil)
	if len(result) != len(tests) {
		t.Fatalf("result has %d tests, want %d", len(result), len(tests))
	}
	for i := range tests {
		if result[i] != tests[i] {
			t.Fatalf("result[%d] = %#v, want %#v", i, result[i], tests[i])
		}
	}
}

func TestFilterTestDurations_PartialExclusion(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "github.com/example/production", Test: "TestFast", Elapsed: 1.0},
		{Package: "github.com/example/features", Test: "TestFeature", Elapsed: 2.0},
		{Package: "github.com/example/integration", Test: "TestIntegration", Elapsed: 3.0},
	}
	result := FilterTestDurations(tests, []string{"features", "integration"})
	if len(result) != 1 {
		t.Fatalf("result has %d tests, want 1: %#v", len(result), result)
	}
	if result[0].Name() != "github.com/example/production.TestFast" || result[0].Elapsed != 1.0 {
		t.Fatalf("result[0] = %#v, want production test", result[0])
	}
}

func TestFilterTestDurations_AllExcluded(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "github.com/example/integration", Test: "TestIntegration", Elapsed: 47.0},
	}
	result := FilterTestDurations(tests, []string{"integration"})
	if result != nil {
		t.Fatalf("expected nil when all tests excluded, got %v", result)
	}
}

func TestFilterTestDurations_EmptyInput(t *testing.T) {
	t.Parallel()
	result := FilterTestDurations([]TestDuration{}, []string{"integration"})
	if result != nil {
		t.Fatalf("expected nil for empty input, got %v", result)
	}
}
