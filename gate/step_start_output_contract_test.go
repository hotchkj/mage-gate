// Vision: Step start-line contract tests: deterministic labels and qualifier disambiguation.
package gate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestStepStartLine_CoveredTestRejectsTagArgsBeforeStart(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	var displayOut bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &bytes.Buffer{})
	store := NewArtifactStore()
	pkgScope := mustNewPackageScope(t, "./...")
	scope := mustNewQualityScope(t, "./...")
	invRunner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	inv := mustQualityScopeInventoryForTests(t, invRunner, store, mem, fakeTestModuleRoot, scope)

	_, err := CoveredTest(
		context.Background(),
		runner,
		store,
		mem,
		fakeTestModuleRoot,
		pkgScope,
		scope,
		inv,
		TestArgs("-tags=integration"),
	)
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected CoveredTest TestArgs tags to be rejected, got %v", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != "" {
		t.Fatalf("expected no start line before tag rejection, got %q", got)
	}
}

func TestStepStartLine_CoveredTestPrefersQualityScopeTags(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	var displayOut bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &bytes.Buffer{})
	store := NewArtifactStore()
	pkgScope := mustNewPackageScope(t, "./...")
	scope, err := NewQualityScope("./...", Tags("integration"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	invRunner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	inv := mustQualityScopeInventoryForTests(t, invRunner, store, mem, fakeTestModuleRoot, scope)

	_, err = CoveredTest(
		context.Background(),
		runner,
		store,
		mem,
		fakeTestModuleRoot,
		pkgScope,
		scope,
		inv,
	)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	if got := strings.TrimSpace(displayOut.String()); got != "Covered Test [tags=integration]..." {
		t.Fatalf("expected covered test tagged line, got %q", got)
	}
}

func TestStepStartLine_CoverageAndDurationInheritQualifier(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	var displayOut bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &bytes.Buffer{})
	store := NewArtifactStore()
	pkgScope := mustNewPackageScope(t, "./...")
	scope, err := NewQualityScope("./...", Tags("unit"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	invRunner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	inv := mustQualityScopeInventoryForTests(t, invRunner, store, mem, fakeTestModuleRoot, scope)

	coveredOut, err := CoveredTest(
		context.Background(),
		runner,
		store,
		mem,
		fakeTestModuleRoot,
		pkgScope,
		scope,
		inv,
	)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}
	testOut, err := coveredOut.TestRun()
	if err != nil {
		t.Fatalf("TestRun() failed: %v", err)
	}
	_, err = Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, coveredOut, MinPercent(0))
	if err != nil {
		t.Fatalf("Coverage() failed: %v", err)
	}
	err = Duration(context.Background(), runner, store, mem, fakeTestModuleRoot, testOut, MaxSeconds(1))
	if err != nil {
		t.Fatalf("Duration() failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(displayOut.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 start lines, got %d: %q", len(lines), displayOut.String())
	}
	if lines[0] != "Covered Test [tags=unit]..." {
		t.Fatalf("covered test line mismatch: %q", lines[0])
	}
	if lines[1] != "Coverage [tags=unit]..." {
		t.Fatalf("coverage line mismatch: %q", lines[1])
	}
	if lines[2] != "Duration [tags=unit]..." {
		t.Fatalf("duration line mismatch: %q", lines[2])
	}
}
