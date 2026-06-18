//go:build integration

package examplecheck

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	qg "github.com/hotchkj/mage-gate/gate"
)

func assertErrorFixHintTuple(t *testing.T, err error, wantName string) {
	t.Helper()
	de := requireDiagnosticError(t, err, wantName)
	assertDiagnosticErrorTuple(t, de, wantName)
}

func assertDiagnosticErrorTuple(t *testing.T, de *qg.DiagnosticError, wantName string) {
	t.Helper()

	text := de.Error()
	if de.Name() != wantName {
		t.Fatalf("expected diagnostic name %q, got %q", wantName, de.Name())
	}
	if de.Message() == "" {
		t.Fatalf("missing diagnostic message in error output: %q", text)
	}
	if de.Fix() == "" {
		t.Fatalf("missing Fix block in diagnostic: %q", text)
	}
	if de.Hint() == "" {
		t.Fatalf("missing Hint block in diagnostic: %q", text)
	}
	if err := assertDiagnosticBlocksOnString(text); err != nil {
		t.Fatal(err)
	}
}

func assertDiagnosticBlocksOnString(text string) error {
	errorLine := lineNumberOf(text, "ERROR:")
	fixLine := lineNumberOf(text, "Fix:")
	hintLine := lineNumberOf(text, "Hint:")
	if errorLine < 0 {
		return fmt.Errorf("%w: expected ERROR block in error output", errDiagnosticOutputFormat)
	}
	if fixLine < 0 {
		return fmt.Errorf("%w: expected Fix block in error output", errDiagnosticOutputFormat)
	}
	if hintLine < 0 {
		return fmt.Errorf("%w: expected Hint block in error output", errDiagnosticOutputFormat)
	}
	if errorLine >= fixLine || fixLine >= hintLine {
		return fmt.Errorf(
			"%w: diagnostic blocks in wrong order: ERROR line %d, Fix line %d, Hint line %d",
			errDiagnosticOutputFormat,
			errorLine,
			fixLine,
			hintLine,
		)
	}
	return nil
}

func lineNumberOf(text, prefix string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return i
		}
	}
	return -1
}

type mutationCoverageMetrics struct {
	percent    float64
	threshold  int
	covered    int
	total      int
	notCovered int
}

func assertMutationCoverageDiagnosticContract(
	t *testing.T,
	err error,
	wantName string,
	wantThreshold int,
	wantCovered int,
	wantTotal int,
	wantNotCovered int,
) {
	t.Helper()
	de := requireDiagnosticError(t, err, wantName)
	assertDiagnosticErrorTuple(t, de, wantName)
	const mutationCoverageMessagePattern = `^mutation coverage (\d+\.\d+)% below threshold (\d+)%` +
		` \((\d+) of (\d+) mutants covered;` +
		` (\d+) not covered by test profile\)$`
	gotMetrics, parseErr := parseMutationCoverageDiagnosticMetrics(de.Message(), mutationCoverageMessagePattern)
	if parseErr != nil {
		t.Fatalf("expected mutation-coverage threshold message, got %q: %v", de.Message(), parseErr)
	}
	verifyMutationCoverageTotals(t, gotMetrics, wantThreshold, wantCovered, wantTotal, wantNotCovered)
	if gotMetrics.percent < 0 {
		t.Fatal("mutation coverage percent must be non-negative")
	}
	if gotMetrics.total > 0 {
		validateMutationCoveragePercent(t, gotMetrics, wantThreshold)
	}
	validateMutationCoverageText(t, de)
	if de.ToolOutput() == "" {
		t.Fatalf("expected mutation coverage tool output, got %q", de.ToolOutput())
	}
}

func verifyMutationCoverageTotals(
	t *testing.T,
	gotMetrics mutationCoverageMetrics,
	wantThreshold, wantCovered, wantTotal, wantNotCovered int,
) {
	t.Helper()
	if gotMetrics.threshold != wantThreshold {
		t.Fatalf("expected mutationcoverage threshold %d, got %d", wantThreshold, gotMetrics.threshold)
	}
	if gotMetrics.covered != wantCovered {
		t.Fatalf("expected covered mutants %d, got %d", wantCovered, gotMetrics.covered)
	}
	if gotMetrics.total != wantTotal {
		t.Fatalf("expected total mutants %d, got %d", wantTotal, gotMetrics.total)
	}
	if gotMetrics.notCovered != wantNotCovered {
		t.Fatalf("expected not-covered mutants %d, got %d", wantNotCovered, gotMetrics.notCovered)
	}
	if gotMetrics.covered+gotMetrics.notCovered != gotMetrics.total {
		t.Fatalf(
			"coverage counters inconsistent: covered=%d notCovered=%d total=%d",
			gotMetrics.covered,
			gotMetrics.notCovered,
			gotMetrics.total,
		)
	}
}

func validateMutationCoveragePercent(t *testing.T, gotMetrics mutationCoverageMetrics, wantThreshold int) {
	t.Helper()
	if gotMetrics.total > 0 {
		wantPercent := float64(gotMetrics.covered) / float64(gotMetrics.total) * 100.0
		tolerance := 0.05
		if gotMetrics.percent < wantPercent-tolerance || gotMetrics.percent > wantPercent+tolerance {
			t.Fatalf(
				"expected coverage percent %.2f from %d/%d, got %.2f",
				wantPercent,
				gotMetrics.covered,
				gotMetrics.total,
				gotMetrics.percent,
			)
		}
	}
	if gotMetrics.percent >= float64(wantThreshold) {
		t.Fatalf("expected failure percentage below threshold: got %.2f >= %d", gotMetrics.percent, wantThreshold)
	}
}

func validateMutationCoverageText(t *testing.T, de *qg.DiagnosticError) {
	t.Helper()
	const wantFix = "expand the go test coverage profile (coverpkg) so more mutation points are considered covered"
	if de.Fix() != wantFix {
		t.Fatalf("expected mutation coverage Fix = %q, got %q", wantFix, de.Fix())
	}
	const wantHint = "NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages"
	if de.Hint() != wantHint {
		t.Fatalf("expected mutation coverage Hint = %q, got %q", wantHint, de.Hint())
	}
	if de.ToolOutput() == "" {
		t.Fatalf("expected mutation coverage tool output, got %q", de.ToolOutput())
	}
}

func parseMutationCoverageDiagnosticMetrics(message, pattern string) (metrics mutationCoverageMetrics, parseErr error) {
	const wrappedPrefix = "mutationcoverage failed: "
	message = strings.TrimPrefix(message, wrappedPrefix)
	matches := regexp.MustCompile(pattern).FindStringSubmatch(message)
	if len(matches) != 6 {
		return metrics, fmt.Errorf("%w: got %q", errDiagnosticOutputFormat, message)
	}
	metrics.percent, parseErr = strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage percent %q: %w", matches[1], parseErr)
	}
	thresholdValue, parseErr := strconv.Atoi(matches[2])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage threshold %q: %w", matches[2], parseErr)
	}
	metrics.covered, parseErr = strconv.Atoi(matches[3])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage covered %q: %w", matches[3], parseErr)
	}
	metrics.total, parseErr = strconv.Atoi(matches[4])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage total %q: %w", matches[4], parseErr)
	}
	metrics.notCovered, parseErr = strconv.Atoi(matches[5])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage not covered %q: %w", matches[5], parseErr)
	}
	metrics.threshold = thresholdValue
	return metrics, nil
}

func TestMutationCoverageFailure_HasStructuredDiagnosticContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	dir := filepath.Join(root, "testdata", "failures", "mutation-fail")
	requireFixtureDir(t, dir)

	runErr := withChdir(dir, func() error {
		scanOut, scanErr := mutationScanForFixture(t, ctx)
		if scanErr != nil {
			return scanErr
		}
		return qg.MutationCoverage(scanOut, qg.MinMutationCoverage(90))
	})
	if runErr == nil {
		t.Fatal("expected mutation coverage step to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	assertMutationCoverageDiagnosticContract(t, runErr, "mutationcoverage", 90, 0, 4, 4)
}

func TestFixtureFailures_AgentModeContract_AllSteps(t *testing.T) {
	root := fixtureFailuresRepoRoot(t)
	for _, tc := range fixtureFailureContractCases(t, root) {
		t.Run(tc.stepName, runFixtureFailureContract(tc))
	}
}

func fixtureFailureContractCases(t *testing.T, root string) []fixtureFailureStep {
	t.Helper()
	return []fixtureFailureStep{
		fixtureFailureCaseLint(t, root),
		fixtureFailureCaseDeadcode(t, root),
		fixtureFailureCaseCompile(t, root),
		fixtureFailureCaseTest(t, root),
		fixtureFailureCaseCoverage(t, root),
		fixtureFailureCaseCrap(t, root),
		fixtureFailureCaseDuration(t, root),
		fixtureFailureCaseMutationSites(t, root),
		fixtureFailureCaseMutationCoverage(t, root),
		fixtureFailureCaseMutationKills(t, root),
	}
}

func fixtureFailureCaseLint(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "lint-fail"),
		stepName:   "lint",
		goldenBase: "lint",
		runInSilent: func(ctx context.Context) error {
			runner, resolver, _, fileOps, pathRoot := newProductionWiring(t)
			pkgScope, err := qg.NewPackageScope("./...")
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			return qg.Lint(
				ctx, runner, resolver, fileOps, pathRoot, pkgScope,
				mustLintToolchain(t, qg.LintConfig(".golangci.yml"), qg.LintToolSpec(lintToolSpec)),
			)
		},
	}
}

func fixtureFailureCaseDeadcode(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "deadcode-fail"),
		stepName:   "deadcode",
		goldenBase: "deadcode",
		runInSilent: func(ctx context.Context) error {
			runner, resolver, _, fileOps, pathRoot := newProductionWiring(t)
			pkgScope, err := qg.NewPackageScope("./...")
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			return qg.Deadcode(
				ctx, runner, resolver, fileOps, pathRoot, pkgScope,
				qg.DeadcodeToolSpec(deadcodeToolSpec), qg.DeadcodeArgs("-tags="+qg.DeadcodeRootsBuildTag),
			)
		},
	}
}

func fixtureFailureCaseCompile(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "build-fail"),
		stepName:   "compile",
		goldenBase: "compile",
		runInSilent: func(ctx context.Context) error {
			runner, _, _, fileOps, pathRoot := newProductionWiring(t)
			pkgScope, err := qg.NewPackageScope("./...")
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			return qg.Compile(ctx, runner, fileOps, pathRoot, pkgScope)
		},
	}
}

func fixtureFailureCaseTest(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "test-fail"),
		stepName:   "test",
		goldenBase: "test",
		runInSilent: func(ctx context.Context) error {
			runner, _, store, fileOps, pathRoot := newProductionWiring(t)
			scope, err := qg.NewQualityScope("./...")
			if err != nil {
				return fmt.Errorf("scope: %w", err)
			}
			pkgScope, err := qg.NewPackageScope(scope.Packages())
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			_, err = qg.Test(ctx, runner, store, fileOps, pathRoot, pkgScope)
			return err
		},
	}
}

func fixtureFailureCaseCoverage(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "coverage-fail"),
		stepName:   "coverage",
		goldenBase: "coverage",
		runInSilent: func(ctx context.Context) error {
			runner, _, store, fileOps, pathRoot := newProductionWiring(t)
			scope, err := qg.NewQualityScope("./...")
			if err != nil {
				return fmt.Errorf("scope: %w", err)
			}
			pkgScope, err := qg.NewPackageScope("./...")
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, pathRoot, scope)
			if err != nil {
				return err
			}
			unitCov, err := qg.CoveredTest(ctx, runner, store, fileOps, pathRoot, pkgScope, scope, inv)
			if err != nil {
				return err
			}
			_, err = qg.Coverage(ctx, runner, store, fileOps, pathRoot, unitCov, qg.MinPercent(90))
			return err
		},
	}
}

func fixtureFailureCaseCrap(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "crap-fail"),
		stepName:   "crap",
		goldenBase: "crap",
		runInSilent: func(ctx context.Context) error {
			runner, resolver, store, fileOps, pathRoot := newProductionWiring(t)
			scope, err := qg.NewQualityScope("./...")
			if err != nil {
				return fmt.Errorf("scope: %w", err)
			}
			pkgScope, err := qg.NewPackageScope("./...")
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, pathRoot, scope)
			if err != nil {
				return err
			}
			unitCov, err := qg.CoveredTest(ctx, runner, store, fileOps, pathRoot, pkgScope, scope, inv)
			if err != nil {
				return err
			}
			covOut, err := qg.Coverage(ctx, runner, store, fileOps, pathRoot, unitCov, qg.MinPercent(0))
			if err != nil {
				return err
			}
			return qg.Crap(
				ctx, runner, resolver, store, fileOps, pathRoot, covOut, inv, qg.MaxScore(0.1),
				qg.GocycloToolSpec("github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"),
			)
		},
	}
}

func fixtureFailureCaseDuration(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "duration-fail"),
		stepName:   "duration",
		goldenBase: "duration",
		runInSilent: func(ctx context.Context) error {
			runner, _, store, fileOps, pathRoot := newProductionWiring(t)
			scope, err := qg.NewQualityScope("./...")
			if err != nil {
				return fmt.Errorf("scope: %w", err)
			}
			pkgScope, err := qg.NewPackageScope(scope.Packages())
			if err != nil {
				return fmt.Errorf("package scope: %w", err)
			}
			testOut, err := qg.Test(ctx, runner, store, fileOps, pathRoot, pkgScope, qg.TestArgs("-count=1"))
			if err != nil {
				return err
			}
			return qg.Duration(ctx, runner, store, fileOps, pathRoot, testOut, qg.MaxSeconds(0.001))
		},
	}
}

func fixtureFailureCaseMutationSites(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "mutation-fail"),
		stepName:   "mutationsites",
		goldenBase: "mutationsites",
		runInSilent: func(ctx context.Context) error {
			scanOut, err := mutationScanForFixture(t, ctx)
			if err != nil {
				return err
			}
			return qg.MutationSites(scanOut, qg.MaxSites(1))
		},
	}
}

func fixtureFailureCaseMutationCoverage(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "mutation-fail"),
		stepName:   "mutationcoverage",
		goldenBase: "mutationcoverage",
		runInSilent: func(ctx context.Context) error {
			scanOut, err := mutationScanForFixture(t, ctx)
			if err != nil {
				return err
			}
			return qg.MutationCoverage(scanOut, qg.MinMutationCoverage(90))
		},
	}
}

func fixtureFailureCaseMutationKills(t *testing.T, root string) fixtureFailureStep {
	return fixtureFailureStep{
		fixtureDir: filepath.Join(root, "testdata", "failures", "mutation-fail"),
		stepName:   "mutationkills",
		goldenBase: "mutationkills",
		runInSilent: func(ctx context.Context) error {
			runner, resolver, store, fileOps, pathRoot := newProductionWiring(t)
			scope, err := qg.NewQualityScope("./...")
			if err != nil {
				return fmt.Errorf("scope: %w", err)
			}
			inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, pathRoot, scope)
			if err != nil {
				return err
			}
			_, err = qg.MutationKills(
				ctx, runner, resolver, store, fileOps, pathRoot,
				scope, inv, qg.MinKillRate(90), qg.GremlinsToolSpec(gremlinsToolSpec),
			)
			return err
		},
	}
}
