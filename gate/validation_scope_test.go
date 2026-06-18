// Vision: Coverage and Crap accept valid scope configs: zero-value scope, packages-only scope, split run/measure scope.
package gate

import (
	"context"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

const validCoverageProfileBody = "mode: set\nexample.com/mod/pkg/file.go:1.1,2.2 3 1\n"

func TestCoverageZeroValueScope(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	pkgOnly, err := NewPackageScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	validOutput := CoveredTestOutput{
		stepID:       "test-step-id",
		packages:     pkgOnly,
		qualityScope: mustNewQualityScope(t, stepsTestScope),
	}
	writeErr := store.Write(
		validOutput.stepID,
		"coverage.out",
		[]byte(validCoverageProfileBody),
		Provenance{Tool: "test"},
	)
	if writeErr != nil {
		t.Fatalf("store.Write: %v", writeErr)
	}
	_, err = Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, validOutput, MinPercent(90))
	if err != nil {
		t.Fatalf("expected no error for valid token with packages-only scope, got %v", err)
	}
}

func TestCrapZeroValueScope(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	validOutput := CoverageOutput{
		stepID:       "coverage-step-id",
		qualityScope: mustNewQualityScope(t, stepsTestScope),
	}
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, fakeTestModuleRoot, validOutput.qualityScope)
	writeErr := store.Write(
		validOutput.stepID,
		"coverage.out",
		[]byte(validCoverageProfileBody),
		Provenance{Tool: "test"},
	)
	if writeErr != nil {
		t.Fatalf("store.Write: %v", writeErr)
	}
	rslv := gatetest.NewFakeToolResolver()
	err := Crap(
		context.Background(),
		runner,
		rslv,
		store,
		mem,
		fakeTestModuleRoot,
		validOutput,
		inv,
		MaxScore(8),
		testGocycloTool,
	)
	if err != nil {
		t.Fatalf("expected no error for valid token with packages-only scope, got %v", err)
	}
}

func TestCoverageValidScope(t *testing.T) {
	scope, err := NewQualityScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	validOutput := CoveredTestOutput{
		stepID:       "test-step-id",
		packages:     pkgScope,
		qualityScope: scope,
	}
	writeErr := store.Write(
		validOutput.stepID,
		"coverage.out",
		[]byte(validCoverageProfileBody),
		Provenance{Tool: "test"},
	)
	if writeErr != nil {
		t.Fatalf("store.Write: %v", writeErr)
	}
	_, err = Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, validOutput, MinPercent(90))
	if err != nil {
		t.Fatalf("expected no error for valid token with valid scope, got %v", err)
	}
}

func TestCoveredTestOutputSplitRunTargetVsMeasurementScope(t *testing.T) {
	t.Parallel()
	pkg := mustNewPackageScope(t, "./cmd/...")
	qs := mustNewQualityScope(t, "./...")
	coveredOut := CoveredTestOutput{
		stepID:       "split-scope-step",
		packages:     pkg,
		qualityScope: qs,
	}
	testOut, err := coveredOut.TestRun()
	if err != nil {
		t.Fatalf("TestRun: %v", err)
	}
	if testOut.scope.Packages() != "./cmd/..." {
		t.Fatalf("Duration token run target: got %q, want ./cmd/...", testOut.scope.Packages())
	}

	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	writeErr := store.Write(
		coveredOut.stepID,
		"coverage.out",
		[]byte(validCoverageProfileBody),
		Provenance{Tool: "test"},
	)
	if writeErr != nil {
		t.Fatalf("store.Write: %v", writeErr)
	}
	covOut, err := Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, coveredOut, MinPercent(90))
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	assertQualityScopesEqual(t, covOut.qualityScope, qs)
}

func TestCrapValidScope(t *testing.T) {
	scope, err := NewQualityScope(stepsTestScope)
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	validOutput := CoverageOutput{
		stepID:       "coverage-step-id",
		qualityScope: scope,
	}
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, fakeTestModuleRoot, scope)
	writeErr := store.Write(validOutput.stepID, "coverage.out", []byte("mode: count\n"), Provenance{Tool: "test"})
	if writeErr != nil {
		t.Fatalf("store.Write: %v", writeErr)
	}
	rslv := gatetest.NewFakeToolResolver()
	err = Crap(
		context.Background(),
		runner,
		rslv,
		store,
		mem,
		fakeTestModuleRoot,
		validOutput,
		inv,
		MaxScore(8),
		testGocycloTool,
	)
	if err != nil {
		t.Fatalf("expected no error for valid token with valid scope, got %v", err)
	}
}
