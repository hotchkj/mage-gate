// Vision: Exported tests for unexported Crap prerequisites—package CRAP scores require coverage on those helpers.
package gate

import (
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestCrapValidateCoveragePrerequisites(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	store := NewArtifactStore()
	cov := CoverageOutput{stepID: "s1", qualityScope: scope}

	if err := crapValidateCoveragePrerequisites(store, &cov); err == nil {
		t.Fatal("expected error when coverage.out is missing")
	}

	if err := store.Write("s1", "coverage.out", []byte("mode: count\n"), Provenance{Tool: "test"}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}
	if err := crapValidateCoveragePrerequisites(store, &cov); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCrapValidateCore_RejectsNilRunner(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	cov := CoverageOutput{stepID: "s1", qualityScope: scope}
	err = crapValidateCore(nil, gatetest.NewFakeToolResolver(), mem, store, &cov, MaxScore(1))
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}
