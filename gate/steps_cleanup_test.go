// Vision: Harness cleanup errors follow the same output-mode shaping as step failures.
package gate

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

var errHarnessCleanupForced = errors.New("forced harness cleanup failure")

// removeAllFailingFileOps delegates to inner except RemoveAll, which always fails
// (exercises harness Cleanup after a successful step).
type removeAllFailingFileOps struct {
	inner FileOps
}

func (r *removeAllFailingFileOps) Root(root string) error {
	return r.inner.Root(root)
}

func (r *removeAllFailingFileOps) MkdirAll(path string, perm fs.FileMode) error {
	return r.inner.MkdirAll(path, perm)
}

func (r *removeAllFailingFileOps) MkdirTemp(dir, pattern string) (string, error) {
	return r.inner.MkdirTemp(dir, pattern)
}

func (r *removeAllFailingFileOps) RemoveAll(path string) error {
	return errHarnessCleanupForced
}

func (r *removeAllFailingFileOps) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return r.inner.WriteFile(path, data, perm)
}

func (r *removeAllFailingFileOps) ReadFile(path string) ([]byte, error) {
	return r.inner.ReadFile(path)
}

func (r *removeAllFailingFileOps) CreateFile(path string) (io.WriteCloser, error) {
	return r.inner.CreateFile(path)
}

func (r *removeAllFailingFileOps) Walk(root string, fn filepath.WalkFunc) error {
	return r.inner.Walk(root, fn)
}

// testMutationKillsGremlinsReport is minimal valid gremlins JSON for MutationKills with MinKillRate(0).
var testMutationKillsGremlinsReport = []byte(
	`{"files":[{"file_name":"pkg/m.go","mutations":[{"status":"KILLED"}]}]}`,
)

// harnessCleanupRunFunc runs one public step using backing vs failing file ops for prerequisite isolation.
type harnessCleanupRunFunc func(
	tb *testing.T,
	runner CommandRunner,
	backing *gatetest.MemoryFileOps,
	failing FileOps,
	store *ArtifactStore,
	inner *cmdtest.FakeRunner,
) error

type harnessCleanupCase struct {
	name           string
	wantStepName   string
	buildInner     func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner
	runWithFailing harnessCleanupRunFunc
}

func assertHarnessCleanupErrorShaping(t *testing.T, wantStepName string, silentErr, verboseErr error) {
	t.Helper()
	if silentErr == nil {
		t.Fatal("silent display: expected error from harness cleanup")
	}
	var de *DiagnosticError
	if !errors.As(silentErr, &de) {
		t.Fatalf("silent display: expected *DiagnosticError, got %T: %v", silentErr, silentErr)
	}
	if de.Name() != wantStepName {
		t.Fatalf("silent display: diagnostic name = %q, want %q", de.Name(), wantStepName)
	}
	if !errors.Is(silentErr, errHarnessCleanupForced) {
		t.Fatalf("silent display: expected errors.Is cleanup sentinel, got %v", silentErr)
	}

	if verboseErr == nil {
		t.Fatal("verbose display: expected error from harness cleanup")
	}
	if errors.As(verboseErr, &de) {
		t.Fatalf("verbose display: expected raw error path, got diagnostic: %v", verboseErr)
	}
	if !errors.Is(verboseErr, errHarnessCleanupForced) {
		t.Fatalf("verbose display: expected errors.Is cleanup sentinel, got %v", verboseErr)
	}
}

func harnessCleanupShapingCases(t *testing.T) []harnessCleanupCase {
	t.Helper()
	ctx := context.Background()
	root := fakeTestModuleRoot
	scope := mustNewQualityScope(t, stepsTestScope)
	pkgScope := mustNewPackageScope(t, stepsTestScope)
	cases := harnessCleanupBuildCases(ctx, root, pkgScope)
	cases = append(cases, harnessCleanupTestCases(ctx, root, pkgScope, scope)...)
	cases = append(cases, harnessCleanupAnalysisCases(ctx, root, pkgScope, scope)...)
	cases = append(cases, harnessCleanupMutationCases(ctx, root, scope)...)
	return cases
}

func harnessCleanupBuildCases(ctx context.Context, root string, pkgScope PackageScope) []harnessCleanupCase {
	return []harnessCleanupCase{
		{
			name:         "compile",
			wantStepName: "compile",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, _ *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				return Compile(ctx, runner, failing, root, pkgScope)
			},
		},
		{
			name:         "vet",
			wantStepName: "vet",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem, cmdtest.On("go vet", gatetest.NoopCommand))
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, _ *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				return Vet(ctx, runner, failing, root, pkgScope)
			},
		},
		{
			name:         "lint",
			wantStepName: "lint",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, _ *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				resolver := gatetest.NewFakeToolResolver()
				resolver.SetLocalMatch("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)
				return Lint(ctx, runner, resolver, failing, root, pkgScope, testDefaultLintToolchain(tb))
			},
		},
		{
			name:         "deadcode",
			wantStepName: "deadcode",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return cmdtest.NewFakeRunner(cmdtest.On("deadcode", gatetest.NoopCommand))
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, _ *gatetest.MemoryFileOps,
				failing FileOps, _ *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				resolver := gatetest.NewFakeToolResolver()
				resolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.31.0", true)
				return Deadcode(ctx, runner, resolver, failing, root, pkgScope,
					DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"))
			},
		},
	}
}

func harnessCleanupTestCases(
	ctx context.Context, root string, pkgScope PackageScope, scope QualityScope,
) []harnessCleanupCase {
	return []harnessCleanupCase{
		{
			name:         "test",
			wantStepName: "test",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				_, err := Test(ctx, runner, store, failing, root, pkgScope)
				return err
			},
		},
		{
			name:         "coveredtest",
			wantStepName: "coveredtest",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				_, err := CoveredTest(ctx, runner, store, failing, root, pkgScope, scope, inv)
				return err
			},
		},
	}
}

func harnessCleanupAnalysisCases(
	ctx context.Context, root string, pkgScope PackageScope, scope QualityScope,
) []harnessCleanupCase {
	return []harnessCleanupCase{
		{
			name:         "coverage",
			wantStepName: "coverage",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				unitCov, err := CoveredTest(ctx, runner, store, backing, root, pkgScope, scope, inv)
				if err != nil {
					return err
				}
				token := mustCoveredTestOutput(tb, &unitCov)
				_, err = Coverage(ctx, runner, store, failing, root, token, MinPercent(0))
				return err
			},
		},
		{
			name:         "crap",
			wantStepName: "crap",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem, cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})))
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				unitCov, err := CoveredTest(ctx, runner, store, backing, root, pkgScope, scope, inv)
				if err != nil {
					return err
				}
				token := mustCoveredTestOutput(tb, &unitCov)
				covOut, err := Coverage(ctx, runner, store, backing, root, token, MinPercent(0))
				if err != nil {
					return err
				}
				resolver := gatetest.NewFakeToolResolver()
				resolver.SetLocalMatch("gocyclo", testGocycloToolSpec, true)
				return Crap(ctx, runner, resolver, store, failing, root, covOut, inv, MaxScore(100), testGocycloTool)
			},
		},
		{
			name:         "duration",
			wantStepName: "duration",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				unitCov, err := CoveredTest(ctx, runner, store, backing, root, pkgScope, scope, inv)
				if err != nil {
					return err
				}
				return Duration(ctx, runner, store, failing, root, mustTestOutputFromCovered(tb, &unitCov), MaxSeconds(3600))
			},
		},
	}
}

func harnessCleanupMutationCases(ctx context.Context, root string, scope QualityScope) []harnessCleanupCase {
	return []harnessCleanupCase{
		{
			name:         "mutationsites",
			wantStepName: "mutationscan",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem)
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				resolver := gatetest.NewFakeToolResolver()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				mr, newErr := NewMutationRunner(runner, resolver, store, failing)
				if newErr != nil {
					return newErr
				}
				scanOut, scanErr := mr.Scan(ctx, root, scope, inv, testGremlinsTool)
				if scanErr != nil {
					return scanErr
				}
				return MutationSites(scanOut, MaxSites(1000))
			},
		},
		{
			name:         "mutationkills",
			wantStepName: "mutationkills",
			buildInner: func(mem *gatetest.MemoryFileOps) *cmdtest.FakeRunner {
				return gateStepFakeRunner(mem,
					cmdtest.On("go run "+testGremlinsToolSpec, gatetest.Gremlins(mem, root, testMutationKillsGremlinsReport)))
			},
			runWithFailing: func(
				tb *testing.T, runner CommandRunner, backing *gatetest.MemoryFileOps,
				failing FileOps, store *ArtifactStore, _ *cmdtest.FakeRunner,
			) error {
				tb.Helper()
				resolver := gatetest.NewFakeToolResolver()
				inv := mustQualityScopeInventoryForTests(tb, runner, store, backing, root, scope)
				_, err := MutationKills(ctx, runner, resolver, store, failing, root, scope, inv, MinKillRate(0), testGremlinsTool)
				return err
			},
		},
	}
}

func TestHarnessCleanupUsesOutputModeShaping(t *testing.T) {
	t.Parallel()
	for _, tc := range harnessCleanupShapingCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := gatetest.NewMemoryFileOps()
			failingFileOps := &removeAllFailingFileOps{inner: mem}
			inner := tc.buildInner(mem)
			store := NewArtifactStore()

			silentRunner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
			silentErr := tc.runWithFailing(t, silentRunner, mem, failingFileOps, store, inner)

			mem2 := gatetest.NewMemoryFileOps()
			failingFileOps2 := &removeAllFailingFileOps{inner: mem2}
			inner2 := tc.buildInner(mem2)
			store2 := NewArtifactStore()
			verboseRunner := mustNewDisplayRunner(t, inner2, OutputModeVerbose, io.Discard, io.Discard)
			verboseErr := tc.runWithFailing(t, verboseRunner, mem2, failingFileOps2, store2, inner2)

			assertHarnessCleanupErrorShaping(t, tc.wantStepName, silentErr, verboseErr)
		})
	}
}

func runCompileCleanupShapingCheck(t *testing.T) {
	t.Helper()
	mem := gatetest.NewMemoryFileOps()
	fileOps := &removeAllFailingFileOps{inner: mem}
	inner := gateStepFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	pkgScope := mustNewPackageScope(t, "./...")
	silentErr := Compile(context.Background(), runner, fileOps, fakeTestModuleRoot, pkgScope)

	mem2 := gatetest.NewMemoryFileOps()
	fileOps2 := &removeAllFailingFileOps{inner: mem2}
	inner2 := gateStepFakeRunner(mem2, cmdtest.On("go build", gatetest.NoopCommand))
	verboseRunner := mustNewDisplayRunner(t, inner2, OutputModeVerbose, io.Discard, io.Discard)
	verboseErr := Compile(context.Background(), verboseRunner, fileOps2, fakeTestModuleRoot, pkgScope)

	assertHarnessCleanupErrorShaping(t, "compile", silentErr, verboseErr)
}

// TestCompileHarnessCleanupErrorSilentDiagnostic and TestCompileHarnessCleanupErrorVerboseRaw are thin
// wrappers around runCompileCleanupShapingCheck so `-run CompileHarnessCleanup` filters
// still exercise cleanup shaping without duplicating the scenario body.
func TestCompileHarnessCleanupErrorSilentDiagnostic(t *testing.T) {
	t.Parallel()
	runCompileCleanupShapingCheck(t)
}

func TestCompileHarnessCleanupErrorVerboseRaw(t *testing.T) {
	t.Parallel()
	runCompileCleanupShapingCheck(t)
}
