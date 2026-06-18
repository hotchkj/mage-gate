// Vision: CoveredTestOutput invariants exercised through Coverage (stale tokens, wrong step IDs)—file split for length.
package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestValidateCoveredTestTokenNil(t *testing.T) {
	t.Parallel()
	err := validateCoveredTestToken(nil)
	if !errors.Is(err, ErrCoveredTestRequired) {
		t.Fatalf("expected ErrCoveredTestRequired, got %v", err)
	}
}

func TestValidateCoveredTestTokenZeroViaPointer(t *testing.T) {
	t.Parallel()
	zero := CoveredTestOutput{}
	err := validateCoveredTestToken(&zero)
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCoverageRejectsEmptyProductionScope(t *testing.T) {
	t.Parallel()
	pkgScope, err := NewPackageScope("./...")
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	out := CoveredTestOutput{
		stepID:       "cov-step",
		packages:     pkgScope,
		qualityScope: QualityScope{},
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", out, MinPercent(90))
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCoverageRejectsEmptyStepIDWithNonEmptyScopes(t *testing.T) {
	t.Parallel()
	pkgScope, err := NewPackageScope("./...")
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	out := CoveredTestOutput{stepID: "", packages: pkgScope, qualityScope: scope}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", out, MinPercent(90))
	assertErrorIs(t, err, ErrMissingValue)
}
