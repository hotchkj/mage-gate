// Vision: Public step surface: nil guards, functional options, and harness wiring without subprocesses.
package gate

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

const (
	stepsTestScope              = "./gate/..."
	fmtStepTestFailed           = "Test() failed: %v"
	fmtStepCoverageFailed       = "Coverage() failed: %v"
	fmtExpectedErrInvalidOption = "expected ErrInvalidOption, got %v"
)

// mustNewDisplayRunner is a test helper that calls NewDisplayRunner and fatals on error.
func mustNewDisplayRunner(tb testing.TB, inner CommandRunner, mode OutputMode, out, err io.Writer) CommandRunner {
	tb.Helper()
	r, e := NewDisplayRunner(inner, mode, out, err)
	if e != nil {
		tb.Fatalf("NewDisplayRunner: %v", e)
	}
	return r
}

// gateStepFakeRunner returns a FakeRunner with responses shared by gate step tests:
// go test (pass), go tool cover, go list, go run (gremlins), and noop golangci-lint.
// Pass additional On() options for step-specific commands.
func gateStepFakeRunner(
	mem gatetest.FileOpsWriter, opts ...cmdtest.RunnerOption,
) *cmdtest.FakeRunner {
	root := fakeTestModuleRoot
	return cmdtest.NewFakeRunner(append([]cmdtest.RunnerOption{
		cmdtest.On("go test", gatetest.GoTestPass(mem, fakeGateGoTestPackage)),
		cmdtest.On("go tool cover", gatetest.GoToolCover(100.0)),
		cmdtest.On("go list", gatetest.GoList(fakeModulePath, root, map[string]gatetest.PackageListInfo{
			fakeGateGoTestPackage: gatetest.DirOnly(filepath.Join(root, "gate")),
			fakeImportPathHarness: gatetest.DirOnly(filepath.Join(root, "internal", "harness")),
		})),
		cmdtest.On("go run", gatetest.Gremlins(mem, root, fakeGremlinsReport)),
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	}, opts...)...)
}

func mustNewLintToolchain(tb testing.TB, config LintConfigValue, tool LintToolValue, opts ...LintOption) LintToolchain {
	tb.Helper()
	lt, err := NewLintToolchain(config, tool, opts...)
	if err != nil {
		tb.Fatalf("NewLintToolchain: %v", err)
	}
	return lt
}

func testDefaultLintToolchain(tb testing.TB) LintToolchain {
	tb.Helper()
	return mustNewLintToolchain(
		tb,
		LintConfig(".golangci.yml"),
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
}

func callsIncludeGolangCILint(calls []cmdrunner.Command) bool {
	for _, c := range calls {
		if c.Name() == "golangci-lint" {
			return true
		}
	}
	return false
}

func TestLintStep(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot

	pkgScope := mustNewPackageScope(t, "./...")
	// Use fake resolver configured to match local golangci-lint binary.
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)

	err := Lint(
		context.Background(),
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		mustNewLintToolchain(
			t,
			LintConfig(".golangci.yml"),
			LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
		),
	)
	if err != nil {
		t.Fatalf("Lint() failed: %v", err)
	}

	// Lint runs golangci-lint directly, not through go.
	if !callsIncludeGolangCILint(inner.Calls()) {
		t.Errorf("expected golangci-lint to be invoked, calls=%v", inner.Calls())
	}
}

func TestFormatStep(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot

	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)

	err := Format(
		context.Background(),
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		mustNewLintToolchain(
			t,
			LintConfig(".golangci.yml"),
			LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
		),
	)
	if err != nil {
		t.Fatalf("Format() failed: %v", err)
	}

	if !callsIncludeGolangCILint(inner.Calls()) {
		t.Errorf("expected golangci-lint to be invoked, calls=%v", inner.Calls())
	}
	foundFmt := false
	for _, c := range inner.Calls() {
		if c.Name() == "golangci-lint" && len(c.Args()) > 0 && c.Args()[0] == "fmt" {
			foundFmt = true
			break
		}
	}
	if !foundFmt {
		t.Errorf("expected golangci-lint fmt subcommand, calls=%v", inner.Calls())
	}
}

func TestCompile(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go build", gatetest.NoopCommand),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(context.Background(), runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Error("expected go build to be called")
	}
}

func TestVetStep(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go vet", gatetest.NoopCommand),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot

	pkgScope := mustNewPackageScope(t, "./...")
	err := Vet(context.Background(), runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Vet() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdVet) {
		t.Error("expected go vet to be called")
	}
}

func TestDeadcodeStep(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot

	pkgScope := mustNewPackageScope(t, "./...")
	// Use fake resolver that does NOT match local deadcode binary, so it falls back to go run.
	fakeResolver := gatetest.NewFakeToolResolver()

	err := Deadcode(
		context.Background(),
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
		DeadcodeArgs("-tags=test"),
	)
	if err != nil {
		t.Fatalf("Deadcode() failed: %v", err)
	}

	// Deadcode runs through go run
	calls := inner.Calls()
	found := slices.ContainsFunc(calls, func(c cmdrunner.Command) bool {
		return c.Name() == "go"
	})
	if !found {
		t.Fatalf("expected a 'go' command call; got %v", calls)
	}
}

func noopGoFakeRunner() *cmdtest.FakeRunner {
	return cmdtest.NewFakeRunner()
}

func TestCoverageRejectsNilStore(t *testing.T) {
	t.Parallel()
	token := CoveredTestOutput{
		stepID:       "some-step",
		packages:     mustNewPackageScope(t, "./..."),
		qualityScope: mustNewQualityScope(t, "./..."),
	}
	mem := gatetest.NewMemoryFileOps()
	_, err := Coverage(
		context.Background(), noopGoFakeRunner(), nil, mem, ".", token, MinPercent(90),
	)
	if err == nil {
		t.Fatal("expected error for nil Store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestCrapRejectsNilStore(t *testing.T) {
	t.Parallel()
	token := CoverageOutput{stepID: "some-step"}
	err := Crap(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		nil,
		gatetest.NewMemoryFileOps(),
		".",
		token,
		QualityScopeInventoryOutput{},
		MaxScore(8),
		testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for nil Store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestDurationRejectsNilStore(t *testing.T) {
	t.Parallel()
	token := TestOutput{stepID: "some-step"}
	err := Duration(
		context.Background(), noopGoFakeRunner(), nil, gatetest.NewMemoryFileOps(),
		".", token, MaxSeconds(5),
	)
	if err == nil {
		t.Fatal("expected error for nil Store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

// TestDurationValidatesOptionsBeforeStore pins the ordering fix from WS5: option
// validation must run before the nil-store check so callers see the most actionable
// error first (misconfiguration vs missing runtime dependency).
func TestDurationValidatesOptionsBeforeStore(t *testing.T) {
	t.Parallel()
	token := TestOutput{stepID: "some-step"}
	// MaxSeconds(0) is invalid; nil store is also invalid.
	// Correct ordering: ErrInvalidOption, not ErrNilDependency.
	err := Duration(
		context.Background(), noopGoFakeRunner(), nil, gatetest.NewMemoryFileOps(),
		".", token, MaxSeconds(0),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption (option validation precedes store check), got %v", err)
	}
}

func TestDurationRejectsEmptyTestOutputPackages(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	token := TestOutput{stepID: "some-step", scope: PackageScope{}}
	err := Duration(context.Background(), noopGoFakeRunner(), store, mem, ".", token, MaxSeconds(5))
	if err == nil {
		t.Fatal("expected error for empty package scope on TestOutput")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestCoverageRejectsZeroCoveredToken(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	_, err := Coverage(context.Background(), runner, store, fileOps, root, CoveredTestOutput{}, MinPercent(90))
	if err == nil {
		t.Fatal("expected error for zero-value CoveredTestOutput")
	}
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCoverageRejectsMissingTestArtifacts(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	token := CoveredTestOutput{
		stepID:       "step-without-coverage-artifact",
		packages:     mustNewPackageScope(t, "./..."),
		qualityScope: mustNewQualityScope(t, "./..."),
	}
	_, err := Coverage(context.Background(), runner, store, fileOps, root, token, MinPercent(90))
	if err == nil {
		t.Fatal("expected error when test step has not populated coverage.out")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestCoverageRejectsInvalidMinPercent(t *testing.T) {
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
	out, err := CoveredTest(ctx, runner, store, fileOps, root, pkgScope, scope, inv)
	if err != nil {
		t.Fatalf(fmtStepTestFailed, err)
	}
	covToken := mustCoveredTestOutput(t, &out)
	_, err = Coverage(ctx, runner, store, fileOps, root, covToken, MinPercent(150))
	if err == nil {
		t.Fatal("expected error for invalid MinPercent on Coverage")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf(fmtExpectedErrInvalidOption, err)
	}
}

func TestCrapRejectsInvalidMaxScore(t *testing.T) {
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
	out, err := CoveredTest(ctx, runner, store, fileOps, root, pkgScope, scope, inv)
	if err != nil {
		t.Fatalf(fmtStepTestFailed, err)
	}
	covToken := mustCoveredTestOutput(t, &out)
	covOut, err := Coverage(ctx, runner, store, fileOps, root, covToken, MinPercent(0))
	if err != nil {
		t.Fatalf(fmtStepCoverageFailed, err)
	}
	err = Crap(ctx, runner, nil, store, fileOps, root, covOut, inv, MaxScore(-1), testGocycloTool)
	if err == nil {
		t.Fatal("expected error for negative MaxScore")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf(fmtExpectedErrInvalidOption, err)
	}
}

func TestCrapRunStepFails(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go run "+testGocycloToolSpec, gatetest.Fail(errForcedFailure)),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	ctx := context.Background()
	scope := mustNewQualityScope(t, stepsTestScope)
	pkgScope := mustNewPackageScope(t, stepsTestScope)
	inv := mustQualityScopeInventoryForTests(t, runner, store, fileOps, fakeRoot, scope)
	out, err := CoveredTest(ctx, runner, store, fileOps, fakeRoot, pkgScope, scope, inv)
	if err != nil {
		t.Fatalf(fmtStepTestFailed, err)
	}
	covToken := mustCoveredTestOutput(t, &out)
	covOut, err := Coverage(ctx, runner, store, fileOps, fakeRoot, covToken, MinPercent(0))
	if err != nil {
		t.Fatalf(fmtStepCoverageFailed, err)
	}
	err = Crap(
		ctx, runner, gatetest.NewFakeToolResolver(), store, fileOps,
		fakeRoot, covOut, inv, MaxScore(8), testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error when Crap step run fails")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if de.Name() != "crap" {
		t.Fatalf("expected crap step in error, got %q", de.Name())
	}
}

func TestDurationRejectsInvalidMaxSeconds(t *testing.T) {
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
	err = Duration(ctx, runner, store, fileOps, root, mustTestOutputFromCovered(t, &unitCov), MaxSeconds(-1))
	if err == nil {
		t.Fatal("expected error for negative MaxSeconds")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf(fmtExpectedErrInvalidOption, err)
	}
}

func TestTestOutputCarriesPackageScope(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	ctx := context.Background()
	root := fakeTestModuleRoot
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	testOut, err := Test(ctx, runner, store, mem, root, pkgScope)
	if err != nil {
		t.Fatalf("Test() failed: %v", err)
	}
	if testOut.scope.Packages() != "./..." {
		t.Fatalf("expected package scope ./..., got %q", testOut.scope.Packages())
	}
}

func TestMutationSitesRejectsInvalidMaxSites(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	st := NewArtifactStore()
	scanOut := MutationScanOutput{
		store:        st,
		stepID:       "mutationscan-1",
		qualityScope: scope,
		outputMode:   OutputModeAgent,
	}
	err = MutationSites(
		scanOut,
		MaxSites(-1),
	)
	if err == nil {
		t.Fatal("expected error for negative MaxSites")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf(fmtExpectedErrInvalidOption, err)
	}
}
