package gate

// Vision: Coverage diagnostics render structured threshold failures without parsing log text.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

var errCoverageMessageFormat = errors.New("expected coverage message format")

func TestCoverageSilentFailureBuildsStructuredDiagnostic(t *testing.T) {
	t.Parallel()
	err := runCoverageFailure(t, OutputModeAgent)
	de := requireCoverageDiagnostic(t, err, OutputModeAgent)
	messagePercent, messageThreshold, parseErr := parseCoverageMessage(de.Message())
	if parseErr != nil {
		t.Fatalf("parse coverage message %q: %v", de.Message(), parseErr)
	}
	if messagePercent != 50.0 || messageThreshold != 90.0 {
		t.Fatalf("unexpected coverage metrics, got percent=%0.1f threshold=%0.1f", messagePercent, messageThreshold)
	}
	assertCoverageFixHint(
		t,
		de,
		"add tests to increase coverage to the required minimum",
		"use go tool cover -func on the coverage profile to find uncovered functions",
	)
	assertCoverageOutputContains(
		t,
		de.ToolOutput(),
		"Worst coverage files:",
		"  50.0%  example.com/mod/pkg/file.go (50/100 statements covered)",
	)
}

func TestCoverageVerboseFailureReturnsRawErrorChain(t *testing.T) {
	t.Parallel()

	err := runCoverageFailure(t, OutputModeVerbose)
	if err == nil {
		t.Fatal("expected coverage failure")
	}
	var de *DiagnosticError
	if errors.As(err, &de) {
		t.Fatalf("expected raw error chain, got diagnostic %v", err)
	}
	if !errors.Is(err, ErrCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrCoverageFailed) must be true, got %v", err)
	}
	raw := err.Error()
	const rawPrefix = "coverage failed: coverage "
	if !strings.HasPrefix(raw, rawPrefix) {
		t.Fatalf("raw error = %q", err)
	}
	trimmed := strings.TrimPrefix(raw, "coverage failed: ")
	rawPercent, rawThreshold, parseErr := parseCoverageMessage(trimmed)
	if parseErr != nil {
		t.Fatalf("parse coverage raw error %q: %v", raw, parseErr)
	}
	if rawPercent != 50.0 || rawThreshold != 90.0 {
		t.Fatalf("unexpected coverage metrics in raw error, got percent=%0.1f threshold=%0.1f", rawPercent, rawThreshold)
	}
}

func requireCoverageDiagnostic(t *testing.T, err error, mode OutputMode) *DiagnosticError {
	t.Helper()
	if err == nil {
		t.Fatal("expected coverage failure")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if !errors.Is(err, ErrCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrCoverageFailed) must be true, got %v", err)
	}
	if mode == OutputModeAgent {
		const wantFix = "add tests to increase coverage to the required minimum"
		const wantHint = "use go tool cover -func on the coverage profile to find uncovered functions"
		assertCoverageFixHint(t, de, wantFix, wantHint)
	}
	return de
}

func assertCoverageFixHint(t *testing.T, de *DiagnosticError, wantFix, wantHint string) {
	t.Helper()
	if de.Fix() != wantFix {
		t.Fatalf("fix should describe coverage action, got %q", de.Fix())
	}
	if de.Hint() != wantHint {
		t.Fatalf("hint should guide coverage investigation, got %q", de.Hint())
	}
}

func assertCoverageOutputContains(t *testing.T, output string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if !hasExactLine(output, line) {
			t.Fatalf("expected coverage output to include %q, got %q", line, output)
		}
	}
}

func TestCoverageSilentNonThresholdFailureUsesGenericDiagnostic(t *testing.T) {
	t.Parallel()

	mem := gatetest.NewMemoryFileOps()
	inner := cmdtest.NewFakeRunner(cmdtest.On("go tool cover", gatetest.GoToolCover(0)))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, stepsTestScope)
	packages := mustNewPackageScope(t, stepsTestScope)
	coveredOut := CoveredTestOutput{
		stepID:       "covered-malformed",
		packages:     packages,
		qualityScope: scope,
	}
	if err := store.Write(coveredOut.stepID, "coverage.out", []byte("mode: set\nmalformed\n"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}

	_, err := Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, coveredOut, MinPercent(90))
	if err == nil {
		t.Fatal("expected malformed coverage failure")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if de.Message() != "coverage failed" {
		t.Fatalf("message = %q, want generic coverage failed", de.Message())
	}
	if !errors.Is(err, ErrCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrCoverageFailed) must be true, got %v", err)
	}
}

func parseCoverageMessage(message string) (percent, threshold float64, err error) {
	re := regexp.MustCompile(`^coverage ([0-9]+(?:\.[0-9]+)?)% \(required >= ([0-9]+(?:\.[0-9]+)?)%\)$`)
	matches := re.FindStringSubmatch(message)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("%w: got %q", errCoverageMessageFormat, message)
	}
	percent, convErr := strconv.ParseFloat(matches[1], 64)
	if convErr != nil {
		return 0, 0, fmt.Errorf("parse percent %q: %w", matches[1], convErr)
	}
	threshold, convErr = strconv.ParseFloat(matches[2], 64)
	if convErr != nil {
		return 0, 0, fmt.Errorf("parse threshold %q: %w", matches[2], convErr)
	}
	return percent, threshold, nil
}

func runCoverageFailure(t *testing.T, mode OutputMode) error {
	t.Helper()
	mem := gatetest.NewMemoryFileOps()
	inner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", gatetest.GoTestPassWithCoverage(mem, fakeGateGoTestPackage, 50)),
		cmdtest.On("go tool cover", gatetest.GoToolCover(50.0)),
		cmdtest.On("go list", gatetest.GoList(fakeModulePath, fakeTestModuleRoot, map[string]gatetest.PackageListInfo{
			fakeGateGoTestPackage: gatetest.DirOnly(filepath.Join(fakeTestModuleRoot, "gate")),
		})),
	)
	runner := mustNewDisplayRunner(t, inner, mode, io.Discard, io.Discard)
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, stepsTestScope)
	packages := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, fakeTestModuleRoot, scope)
	unitCov, err := CoveredTest(context.Background(), runner, store, mem, fakeTestModuleRoot, packages, scope, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}
	_, err = Coverage(context.Background(), runner, store, mem, fakeTestModuleRoot, unitCov, MinPercent(90))
	return err
}
