//go:build integration

package examplecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	qg "github.com/hotchkj/mage-gate/gate"
)

func TestFixtureFailures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	checks := []stepCheck{
		{name: "lint", run: checkLintFailure},
		{name: "deadcode", run: checkDeadcodeFailure},
		{name: "compile", run: checkCompileFailure},
		{name: "test", run: checkTestFailure},
		{name: "coverage", run: checkCoverageFailure},
		{name: "crap", run: checkCrapFailure},
		{name: "duration", run: checkDurationFailure},
		{name: "mutationsites", run: checkMutationSitesFailure},
		{name: "mutationcoverage", run: checkMutationCoverageFailure},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
			defer cancel()
			check.run(t, ctx, root)
		})
	}
}

func newProductionWiring(t *testing.T) (
	runner qg.CommandRunner,
	resolver qg.ToolResolver,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	root string,
) {
	t.Helper()
	runnerCandidate, err := qg.NewDisplayRunner(
		qg.NewProductionRunner(),
		qg.OutputModeAgent,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		t.Fatalf("NewDisplayRunner: %v", err)
	}
	runner = runnerCandidate
	resolver = qg.NewProductionToolResolver()
	store = qg.NewArtifactStore()
	fileOps = qg.NewProductionFileOps()
	root = "."
	return runner, resolver, store, fileOps, root
}

func mustLintToolchain(
	t *testing.T,
	config qg.LintConfigValue,
	tool qg.LintToolValue,
	opts ...qg.LintOption,
) qg.LintToolchain {
	t.Helper()
	lt, err := qg.NewLintToolchain(config, tool, opts...)
	if err != nil {
		t.Fatalf("NewLintToolchain: %v", err)
	}
	return lt
}

func requireFixtureDir(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture directory missing (add under testdata/failures/): %s: %v", dir, err)
	}
}

func skipIfToolingMissing(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, exec.ErrNotFound) {
		t.Skipf("gate tool not on PATH: %v", err)
		return
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, exec.ErrNotFound) {
		t.Skipf("gate tool not on PATH: %v", err)
		return
	}
	// Structured checks above cover all known platforms; string fallbacks removed.
}

func requireDiagnosticError(t *testing.T, err error, wantName string) *qg.DiagnosticError {
	t.Helper()
	var de *qg.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *qg.DiagnosticError, got %T: %v", err, err)
	}
	if de.Name() != wantName {
		t.Fatalf("expected name %q, got %q: %v", wantName, de.Name(), err)
	}
	return de
}

func diagnosticText(de *qg.DiagnosticError) string {
	return de.Message() + "\n" + de.ToolOutput()
}

// toolOutputMentions reports whether tool appears as a distinct token in text
// (after trimming trailing punctuation from each field).
func toolOutputMentions(text, tool string) bool {
	for _, line := range strings.Split(text, "\n") {
		for _, token := range strings.Fields(line) {
			cleaned := strings.Trim(token, ":,;()[]{}")
			cleaned = strings.Trim(cleaned, "`\"'")
			if cleaned == tool {
				return true
			}
		}
	}
	return false
}

// goTestJSONEvent is a minimal view of go test -json stream lines used for assertions.
type goTestJSONEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
}

func toolOutputHasGoTestFailEvent(toolOut string) bool {
	for _, line := range strings.Split(toolOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev goTestJSONEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Action == "fail" && strings.TrimSpace(ev.Test) != "" {
			return true
		}
	}
	return false
}

// toolOutputContainsTestName reports whether a test with testName appears in go test -json output.
func toolOutputContainsTestName(toolOut, testName string) bool {
	for _, line := range strings.Split(toolOut, "\n") {
		var ev struct{ Test string }
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &ev); err != nil {
			continue
		}
		if ev.Test == testName {
			return true
		}
	}
	return false
}

func crapOutputHasRemainderSummary(toolOut string) bool {
	for _, line := range strings.Split(toolOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "... and ") && strings.HasSuffix(line, "offender(s)") {
			return true
		}
	}
	return false
}

// mutationScanForFixture centralizes production wiring, scope, and gremlins dry-run so
// mutation integration checks only differ in the post-scan quality step.
func mutationScanForFixture(t *testing.T, ctx context.Context) (qg.MutationScanOutput, error) {
	t.Helper()
	runner, resolver, store, fileOps, pathRoot := newProductionWiring(t)
	scope, err := qg.NewQualityScope("./...")
	if err != nil {
		return qg.MutationScanOutput{}, fmt.Errorf("scope: %w", err)
	}
	inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, pathRoot, scope)
	if err != nil {
		return qg.MutationScanOutput{}, err
	}
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return qg.MutationScanOutput{}, err
	}
	return mr.Scan(ctx, pathRoot, scope, inv, qg.GremlinsToolSpec(gremlinsToolSpec))
}

func checkLintFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "lint-fail")
	requireFixtureDir(t, dir)
	lintCfg := filepath.Join(dir, ".golangci.yml")
	runErr := withChdir(dir, func() error {
		runner, resolver, _, fileOps, pathRoot := newProductionWiring(t)
		pkgScope, err := qg.NewPackageScope("./...")
		if err != nil {
			return fmt.Errorf("package scope: %w", err)
		}
		return qg.Lint(
			ctx,
			runner,
			resolver,
			fileOps,
			pathRoot,
			pkgScope,
			mustLintToolchain(t, qg.LintConfig(lintCfg), qg.LintToolSpec(lintToolSpec)),
		)
	})
	if runErr == nil {
		t.Fatal("expected lint to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	se := requireDiagnosticError(t, runErr, "lint")
	if !toolOutputMentions(diagnosticText(se), "errcheck") {
		t.Fatalf("expected errcheck in diagnostics, got: %v", runErr)
	}
}

func checkDeadcodeFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "deadcode-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
		runner, resolver, _, fileOps, pathRoot := newProductionWiring(t)
		pkgScope, err := qg.NewPackageScope("./...")
		if err != nil {
			return fmt.Errorf("package scope: %w", err)
		}
		return qg.Deadcode(
			ctx,
			runner,
			resolver,
			fileOps,
			pathRoot,
			pkgScope,
			qg.DeadcodeToolSpec(deadcodeToolSpec),
			qg.DeadcodeArgs("-tags="+qg.DeadcodeRootsBuildTag),
		)
	})
	if runErr == nil {
		t.Fatal("expected deadcode to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	se := requireDiagnosticError(t, runErr, "deadcode")
	if !toolOutputMentions(diagnosticText(se), "unreachable") {
		t.Fatalf("expected unreachable in diagnostics, got: %v", runErr)
	}
	if !toolOutputMentions(diagnosticText(se), "unreachableFunctionFixture") {
		t.Fatalf("expected deadcode identifier detail in diagnostics, got: %v", runErr)
	}
}

func checkCompileFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "build-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
		runner, _, _, fileOps, pathRoot := newProductionWiring(t)
		pkgScope, err := qg.NewPackageScope("./...")
		if err != nil {
			return fmt.Errorf("package scope: %w", err)
		}
		return qg.Compile(ctx, runner, fileOps, pathRoot, pkgScope)
	})
	if runErr == nil {
		t.Fatal("expected compile step to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	se := requireDiagnosticError(t, runErr, "compile")
	if !toolOutputMentions(diagnosticText(se), "undefined") {
		t.Fatalf("expected undefined in diagnostics, got: %v", runErr)
	}
}

func checkTestFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "test-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
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
	})
	if runErr == nil {
		t.Fatal("expected test to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	se := requireDiagnosticError(t, runErr, "test")
	if !toolOutputHasGoTestFailEvent(se.ToolOutput()) {
		t.Fatalf("expected go test -json fail event in diagnostics, got: %v", runErr)
	}
	if toolOutputContainsTestName(se.ToolOutput(), "PASS_EVENT_SPAM") {
		t.Fatalf("expected passing JSON event spam to be filtered, got: %v", runErr)
	}
}

func checkCoverageFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "coverage-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
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
	})
	if runErr == nil {
		t.Fatal("expected coverage gate to fail on fixture")
	}
	assertCoverageDiagnosticContract(t, runErr, "coverage", 0.0, 90.0)
}

func checkDurationFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "duration-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
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
	})
	if runErr == nil {
		t.Fatal("expected duration gate to fail on fixture")
	}
	_ = requireDiagnosticError(t, runErr, "duration")
}

func checkCrapFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "crap-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
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
		// Threshold 0 disables the coverage percent gate; we only need artifacts for CRAP analysis.
		covOut, err := qg.Coverage(ctx, runner, store, fileOps, pathRoot, unitCov, qg.MinPercent(0))
		if err != nil {
			return err
		}
		return qg.Crap(ctx, runner, resolver, store, fileOps, pathRoot, covOut, inv, qg.MaxScore(0.1),
			qg.GocycloToolSpec("github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"))
	})
	if runErr == nil {
		t.Fatal("expected CRAP gate to fail on fixture")
	}
	se := requireDiagnosticError(t, runErr, "crap")
	if !crapOutputHasRemainderSummary(se.ToolOutput()) {
		t.Fatalf("expected truncated CRAP offender summary, got: %v", runErr)
	}
}

func checkMutationSitesFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "mutation-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
		scanOut, err := mutationScanForFixture(t, ctx)
		if err != nil {
			return err
		}
		return qg.MutationSites(scanOut, qg.MaxSites(1))
	})
	if runErr == nil {
		t.Fatal("expected mutation sites gate to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	_ = requireDiagnosticError(t, runErr, "mutationsites")
}

func checkMutationCoverageFailure(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdata", "failures", "mutation-fail")
	requireFixtureDir(t, dir)
	runErr := withChdir(dir, func() error {
		scanOut, err := mutationScanForFixture(t, ctx)
		if err != nil {
			return err
		}
		// No _test.go in the fixture, so the covered/total ratio cannot reach 90%.
		return qg.MutationCoverage(scanOut, qg.MinMutationCoverage(90))
	})
	if runErr == nil {
		t.Fatal("expected mutation coverage gate to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	if !errors.Is(runErr, qg.ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got: %v", runErr)
	}
	assertMutationCoverageDiagnosticContract(t, runErr, "mutationcoverage", 90, 0, 4, 4)
}

func assertCoverageDiagnosticContract(
	t *testing.T,
	err error,
	wantName string,
	wantPercent float64,
	wantThreshold float64,
) {
	t.Helper()
	de := requireDiagnosticError(t, err, wantName)
	assertDiagnosticErrorTuple(t, de, wantName)

	const coverageMessagePattern = `^coverage ([0-9]+(?:\.[0-9]+)?)% \(required >= ([0-9]+(?:\.[0-9]+)?)%\)$`
	re := regexp.MustCompile(coverageMessagePattern)
	matches := re.FindStringSubmatch(de.Message())
	if len(matches) != 3 {
		t.Fatalf("expected coverage message format, got %q", de.Message())
	}
	gotPercent, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		t.Fatalf("failed parsing coverage percent %q: %v", matches[1], parseErr)
	}
	gotThreshold, parseErr := strconv.ParseFloat(strings.TrimSuffix(matches[2], "%"), 64)
	if parseErr != nil {
		t.Fatalf("failed parsing coverage threshold %q: %v", matches[2], parseErr)
	}
	if gotPercent > wantPercent+0.0001 || gotPercent < wantPercent-0.0001 {
		t.Fatalf("expected coverage percent %.2f, got %.2f", wantPercent, gotPercent)
	}
	if gotThreshold != wantThreshold {
		t.Fatalf("expected coverage threshold %.1f, got %.1f", wantThreshold, gotThreshold)
	}
	if gotPercent >= wantThreshold {
		t.Fatalf("expected coverage failure below threshold, got %.2f >= %.1f", gotPercent, wantThreshold)
	}
}
