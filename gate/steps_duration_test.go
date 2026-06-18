// Vision: Duration step—checks all test events from the producing test run.
package gate

import (
	"context"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestDurationPassesAfterCoveredTest(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()
	scope := mustNewQualityScope(t, stepsTestScope)
	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, fileOps, root, scope)
	unitCov, err := CoveredTest(ctx, runner, store, fileOps, root, pkgScope, scope, inv)
	if err != nil {
		t.Fatalf(fmtStepTestFailed, err)
	}
	out := mustTestOutputFromCovered(t, &unitCov)
	err = Duration(ctx, runner, store, fileOps, root, out, MaxSeconds(5))
	if err != nil {
		t.Fatalf("Duration() failed: %v", err)
	}
}
