// Vision: QualityScope package lists and excludes: parsing, normalization, and how downstream steps consume them.
package gate

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestQualityScopeConsistencyAcrossSteps(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot

	qs, err := NewQualityScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs)
	unitCov, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, qs, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	assertQualityScopePackages(t, unitCov.qualityScope)
	assertQualityScopeExcludeCount(t, unitCov.qualityScope, 0)

	covOut, err := Coverage(ctx, runner, store, mem, root, unitCov, MinPercent(90))
	if err != nil {
		t.Fatalf("Coverage() failed: %v", err)
	}

	assertQualityScopePackages(t, covOut.qualityScope)
	assertQualityScopeExcludeCount(t, covOut.qualityScope, 0)

	err = Crap(ctx, runner, gatetest.NewFakeToolResolver(), store, mem, root, covOut, inv, MaxScore(8), testGocycloTool)
	if err != nil {
		t.Fatalf("Crap() failed: %v", err)
	}

	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	scanOut, err := mr.Scan(ctx, root, qs, inv, testGremlinsTool)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := MutationSites(scanOut, MaxSites(50)); err != nil {
		t.Fatalf("MutationSites() failed: %v", err)
	}
}

func TestQualityScopeDriftDetectionDifferentExcludes(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot

	qs1, err := NewQualityScope(stepsTestScope, Exclude("exclude-one"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	qs2, err := NewQualityScope(stepsTestScope, Exclude("exclude-two"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv1 := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs1)
	unitCov, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, qs1, inv1)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	assertQualityScopeExcludeCount(t, unitCov.qualityScope, 1)
	assertQualityScopeExcludeContains(t, unitCov.qualityScope, "exclude-one")

	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	inv2 := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs2)
	scanOut, err := mr.Scan(ctx, root, qs2, inv2, testGremlinsTool)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := MutationSites(scanOut, MaxSites(50)); err != nil {
		t.Fatalf("MutationSites() failed: %v", err)
	}

	assertQualityScopesDifferent(t, qs1, qs2)
}

func TestQualityScopePropagationTestToCoverage(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot

	qs, err := NewQualityScope(stepsTestScope, Exclude("features-x", "testdata-y"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs)
	unitCov, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, qs, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	assertQualityScopePackages(t, unitCov.qualityScope)
	assertQualityScopeExcludeCount(t, unitCov.qualityScope, 2)
}

func TestQualityScopePropagationCoverageToCrap(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot

	qs, err := NewQualityScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs)
	unitCov, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, qs, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	covOut, err := Coverage(ctx, runner, store, mem, root, unitCov, MinPercent(90))
	if err != nil {
		t.Fatalf("Coverage() failed: %v", err)
	}

	assertQualityScopePackages(t, covOut.qualityScope)
	assertQualityScopeExcludeCount(t, covOut.qualityScope, 0)

	err = Crap(ctx, runner, gatetest.NewFakeToolResolver(), store, mem, root, covOut, inv, MaxScore(8), testGocycloTool)
	if err != nil {
		t.Fatalf("Crap() failed: %v", err)
	}
}

func TestMutationSitesUsesSameQualityScopeAsCoverage(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot

	qs, err := NewQualityScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, qs)
	unitCov, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, qs, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	_, err = Coverage(ctx, runner, store, mem, root, unitCov, MinPercent(90))
	if err != nil {
		t.Fatalf("Coverage() failed: %v", err)
	}

	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	scanOut, err := mr.Scan(ctx, root, qs, inv, testGremlinsTool)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := MutationSites(scanOut, MaxSites(50)); err != nil {
		t.Fatalf("MutationSites() failed: %v", err)
	}

	assertQualityScopesEqual(t, unitCov.qualityScope, qs)
}

//nolint:gocritic // Test helper: opaque value token passed by value by design.
func assertQualityScopePackages(t *testing.T, qs QualityScope) {
	t.Helper()
	if qualityScopePackages(qs) != stepsTestScope {
		t.Fatalf("expected quality scope packages '%s', got '%s'", stepsTestScope, qualityScopePackages(qs))
	}
}

//nolint:gocritic // Test helper: opaque value token passed by value by design.
func assertQualityScopeExcludeCount(t *testing.T, qs QualityScope, expected int) {
	t.Helper()
	if len(qs.ExcludeSegments()) != expected {
		t.Fatalf("expected %d exclude segments, got %d", expected, len(qs.ExcludeSegments()))
	}
}

//nolint:gocritic // Test helper: opaque value token passed by value by design.
func assertQualityScopeExcludeContains(t *testing.T, qs QualityScope, segment string) {
	t.Helper()
	for _, gotSegment := range qs.ExcludeSegments() {
		if gotSegment == segment {
			return
		}
	}
	t.Fatalf("expected exclude segment '%s' in %v", segment, qs.ExcludeSegments())
}

//nolint:gocritic // Test helper: opaque value token passed by value by design.
func assertQualityScopesDifferent(t *testing.T, left, right QualityScope) {
	t.Helper()
	if qualityScopePackages(left) == qualityScopePackages(right) &&
		slices.Equal(qualityScopeExcludeSegments(left), qualityScopeExcludeSegments(right)) &&
		slices.Equal(qualityScopeTags(left), qualityScopeTags(right)) &&
		slices.Equal(left.TestFilePatterns(), right.TestFilePatterns()) {
		t.Fatalf("expected different quality scopes:\n"+
			"  left  pkgs=%q excludes=%v tags=%v patterns=%v\n"+
			"  right pkgs=%q excludes=%v tags=%v patterns=%v",
			qualityScopePackages(left), qualityScopeExcludeSegments(left),
			qualityScopeTags(left), left.TestFilePatterns(),
			qualityScopePackages(right), qualityScopeExcludeSegments(right),
			qualityScopeTags(right), right.TestFilePatterns())
	}
}

//nolint:gocritic // Test helper: opaque value token passed by value by design.
func assertQualityScopesEqual(t *testing.T, actual, expected QualityScope) {
	t.Helper()
	if qualityScopePackages(actual) != qualityScopePackages(expected) {
		t.Fatalf("expected same packages, actual='%s', expected='%s'",
			qualityScopePackages(actual), qualityScopePackages(expected))
	}
	actualExcludes := actual.ExcludeSegments()
	expectedExcludes := expected.ExcludeSegments()
	if len(actualExcludes) != len(expectedExcludes) {
		t.Fatalf("expected same number of excludes, actual=%d, expected=%d",
			len(actualExcludes), len(expectedExcludes))
	}
	for i, exclude := range actualExcludes {
		if exclude != expectedExcludes[i] {
			t.Fatalf("expected same exclude at index %d, actual='%s', expected='%s'",
				i, exclude, expectedExcludes[i])
		}
	}
	if !slices.Equal(qualityScopeTags(actual), qualityScopeTags(expected)) {
		t.Fatalf("expected same tags, actual=%v, expected=%v", qualityScopeTags(actual), qualityScopeTags(expected))
	}
}

func TestQualityScopeFingerprintNoDelimiterCollision(t *testing.T) {
	t.Parallel()
	single, err := NewQualityScope("./...", Exclude("a,b"))
	if err != nil {
		t.Fatalf("NewQualityScope single: %v", err)
	}
	double, err := NewQualityScope("./...", Exclude("a", "b"))
	if err != nil {
		t.Fatalf("NewQualityScope double: %v", err)
	}
	fp1 := qualityScopeFingerprint(single)
	fp2 := qualityScopeFingerprint(double)
	if fp1 == "" || fp2 == "" {
		t.Fatal("expected non-empty fingerprints")
	}
	if fp1 == fp2 {
		t.Fatalf("Exclude(\"a,b\") and Exclude(\"a\",\"b\") must produce different fingerprints, both got %q", fp1)
	}
}
