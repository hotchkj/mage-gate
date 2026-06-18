// Vision: CRAP step: join coverage artifacts with gocyclo output and gatecheck scoring under fakes.
//
//nolint:revive // file-length-limit is enforced globally, CRAP harness fixtures are intentionally grouped.
package harness_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

const (
	testHarnessGocycloSpec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"
	goArgvRun              = "run"
)

func isGocycloToolInvocation(c cmdrunner.Command) bool {
	if c.Name() == "gocyclo" {
		return true
	}
	args := c.Args()
	return c.Name() == "go" && len(args) > 0 && args[0] == goArgvRun
}

func findGocycloCallWithOver(calls []cmdrunner.Command) (cmdrunner.Command, bool) {
	for _, cmd := range calls {
		if !isGocycloToolInvocation(cmd) {
			continue
		}
		if !slices.Contains(cmd.Args(), "-over") {
			continue
		}
		return cmd, true
	}
	return cmdrunner.Command{}, false
}

func assertFlagFollowedBy(t *testing.T, args []string, flag, want string) {
	t.Helper()
	idx := slices.Index(args, flag)
	if idx < 0 {
		t.Fatalf("missing %q in args %v", flag, args)
	}
	if idx+1 >= len(args) {
		t.Fatalf("missing value after %q in args %v", flag, args)
	}
	if got := args[idx+1]; got != want {
		t.Fatalf("after %q want %q, got %q in %v", flag, want, got, args)
	}
}

const wantCrapGocycloHostPkgDir = "/test-root/internal/harness"

func assertCrapGocycloInputsAreHostDirs(t *testing.T, runner *cmdtest.FakeRunner) {
	t.Helper()
	cmd, ok := findGocycloCallWithOver(runner.Calls())
	if !ok {
		t.Fatal("expected gocyclo invocation with -over")
	}
	args := cmd.Args()
	if len(args) == 0 {
		t.Fatalf("gocyclo args empty: %#v", args)
	}
	lastDir := args[len(args)-1]
	if filepath.ToSlash(lastDir) != filepath.ToSlash(wantCrapGocycloHostPkgDir) {
		t.Fatalf(
			"gocyclo trailing package dir argv: got %q want %q (full %#v)",
			lastDir, wantCrapGocycloHostPkgDir, args,
		)
	}
}

func assertCrapReportLogicalPathsReadable(
	t *testing.T,
	fops *gatetest.MemoryFileOps,
) {
	t.Helper()
	for _, artifactPath := range []string{testHarnessGocycloLogicalRel, testHarnessCoverFuncLogicalRel} {
		data, err := fops.ReadFile(artifactPath)
		if err != nil {
			t.Fatalf("expected FileOps read at logical artifact path %q: %v", artifactPath, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			t.Fatalf("expected non-empty artifact at logical path %q", artifactPath)
		}
	}
}

func isGoListModuleJSONCall(cmd cmdrunner.Command) bool {
	args := cmd.Args()
	return cmd.Name() == "go" && len(args) >= 3 &&
		args[0] == "list" && slices.Contains(args[1:], "-m") && slices.Contains(args[1:], "-json")
}

func countGoListModuleJSONInvocations(calls []cmdrunner.Command) int {
	count := 0
	for _, cmd := range calls {
		if isGoListModuleJSONCall(cmd) {
			count++
		}
	}
	return count
}

func goListWithModuleJSONStdout(moduleStdout string, packages map[string]gatetest.PackageListInfo) cmdtest.CommandFunc {
	inner := gatetest.GoList("github.com/hotchkj/mage-gate", "/test-root", packages)
	return func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
		if isGoListModuleJSONCall(cmd) {
			_, err := io.WriteString(stdout, moduleStdout)
			return err
		}
		return inner(ctx, cmd, stdout, stderr)
	}
}

func testCrapInventoryRows() []gatecheck.MutationPackageRow {
	return []gatecheck.MutationPackageRow{
		{
			ImportPath:    "github.com/hotchkj/mage-gate/internal/harness",
			PkgDirRootRel: "internal/harness",
		},
	}
}

func testCrapCommandScope(rawExclude string) *gatecheck.QualityScopeCommandScope {
	return testQualityScopeCommandScope(testCrapInventoryRows(), rawExclude, nil, nil)
}

func TestStepCrap_Success(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(), 8.0, testRunStepID, testCrapCommandScope(""), testHarnessGocycloSpec, nil,
	)
	if err != nil {
		t.Fatalf("StepCrap: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
	assertCrapGocycloInputsAreHostDirs(t, runner)
	assertCrapReportLogicalPathsReadable(t, fops)
	if got := countGoListModuleJSONInvocations(runner.Calls()); got != 1 {
		t.Fatalf("want exactly 1 go list -m -json invocation, got %d", got)
	}
}

func TestStepCrap_AppendsCrapArgsAfterOverFlag(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testCrapCommandScope(""),
		testHarnessGocycloSpec,
		[]string{"-top", "10"},
	); err != nil {
		t.Fatalf("StepCrap: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
	assertCrapGocycloInputsAreHostDirs(t, runner)
	assertCrapReportLogicalPathsReadable(t, fops)
	gocycloCall, found := findGocycloCallWithOver(runner.Calls())
	if !found {
		t.Fatal("expected gocyclo invocation")
	}
	args := gocycloCall.Args()
	assertFlagFollowedBy(t, args, "-over", "0")
	assertFlagFollowedBy(t, args, "-top", "10")
}

func TestStepCrap_ModuleMetadataJSONValidationWrapsErrCrapFailed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stdout string
	}{
		{name: "invalid JSON", stdout: "not-json"},
		{name: "missing path", stdout: `{"Path":"","Dir":"/test-root"}`},
		{name: "missing dir", stdout: `{"Path":"github.com/hotchkj/mage-gate","Dir":" "}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fops := gatetest.NewMemoryFileOps()
			store := newMemStore()
			if err := store.Write(testRunStepID, testStoreArtifactCoverage, []byte("mode: set\n"), h.Provenance{}); err != nil {
				t.Fatalf(testFmtStoreWrite, err)
			}
			runner := cmdtest.NewFakeRunner(
				cmdtest.On("go list", goListWithModuleJSONStdout(tc.stdout, map[string]gatetest.PackageListInfo{
					"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
				})),
				cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})),
				cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
					map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
					100.0,
				)),
			)
			deps := validDeps(runner)
			deps.FileOps = fops
			harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
			if err != nil {
				t.Fatalf(testFmtNewHarness, err)
			}
			err = harness.StepCrap(
				context.Background(),
				8.0,
				testRunStepID,
				testCrapCommandScope(""),
				testHarnessGocycloSpec,
				nil,
			)
			if !errors.Is(err, h.ErrCrapFailed) {
				t.Fatalf("StepCrap error = %v, want ErrCrapFailed", err)
			}
			if got := countGoListModuleJSONInvocations(runner.Calls()); got != 1 {
				t.Fatalf("want exactly 1 go list -m -json invocation, got %d", got)
			}
		})
	}
}

func TestStepCrap_FilteredCoverage(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte(testCoverageMixedVendor)
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testCrapCommandScope(testCoverExcludeVendor),
		testHarnessGocycloSpec,
		nil,
	); err != nil {
		t.Fatalf("StepCrap: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageFilteredLogicalRel)
	assertCrapGocycloInputsAreHostDirs(t, runner)
	assertCrapReportLogicalPathsReadable(t, fops)
	wantFilteredProf := "mode: set\ngithub.com/hotchkj/mage-gate/internal/harness/config.go:1.2,3.4 1 1\n"
	filteredBytes, readErr := fops.ReadFile(testHarnessCoverageFilteredLogicalRel)
	if readErr != nil {
		t.Fatalf("read filtered coverage at logical FileOps path %q: %v", testHarnessCoverageFilteredLogicalRel, readErr)
	}
	if string(filteredBytes) != wantFilteredProf {
		t.Fatalf(
			"filtered coverage at %q = %q, want %q",
			testHarnessCoverageFilteredLogicalRel, string(filteredBytes), wantFilteredProf,
		)
	}
}

func TestStepCrap_AllPackagesExcluded(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\ngithub.com/hotchkj/mage-gate/internal/harness/config.go:1.2,3.4 1 1\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testCrapCommandScope("internal"),
		testHarnessGocycloSpec,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when all packages excluded")
	}
	if !errors.Is(err, h.ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
	if !errors.Is(err, gatecheck.ErrAllPackagesExcluded) {
		t.Fatalf("expected ErrAllPackagesExcluded wrapped in error, got %v", err)
	}
}

func TestStepCrap_RejectsPackageDirOutsideRootBeforeGocyclo(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/outside": gatetest.DirOnly("/outside/pkg"),
			})),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 5})),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testQualityScopeCommandScope(
			[]gatecheck.MutationPackageRow{
				{ImportPath: "github.com/hotchkj/mage-gate/outside", PkgDirRootRel: "../outside/pkg"},
			},
			"", nil, nil,
		),
		testHarnessGocycloSpec,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when go list package dir escapes root")
	}
	if !errors.Is(err, h.ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
	if !errors.Is(err, h.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
	if _, found := findGocycloCallWithOver(runner.Calls()); found {
		t.Fatalf("gocyclo must not run after package dir escapes root; calls=%v", runner.Calls())
	}
}

func TestStepCrap_ValidateFails(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Validate": 15})),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 0.0},
			100.0,
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testCrapCommandScope(""),
		testHarnessGocycloSpec,
		nil,
	)
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
	assertCrapGocycloInputsAreHostDirs(t, runner)
	assertCrapReportLogicalPathsReadable(t, fops)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}

func TestStepCrap_RejectsMissingUpstreamStepID(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner()
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCrapID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(), 8.0, "", testCrapCommandScope(""), testHarnessGocycloSpec, nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}

func TestStepCrap_GoRunFallback(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"github.com/hotchkj/mage-gate", "/test-root", map[string]gatetest.PackageListInfo{
				"github.com/hotchkj/mage-gate/internal/harness": gatetest.DirOnly("/test-root/internal/harness"),
			})),
		cmdtest.On("go run", gatetest.Gocyclo(map[string]int{"Validate": 5})),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
	)
	resolver := gatetest.NewFakeToolResolver()
	harness, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir, testPackages,
		runner, fops, store, testStepCrapID,
		h.WithToolResolver(resolver),
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(), 8.0, testRunStepID, testCrapCommandScope(""), testHarnessGocycloSpec, nil,
	)
	if err != nil {
		t.Fatalf("StepCrap: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
	assertCrapGocycloInputsAreHostDirs(t, runner)
	assertCrapReportLogicalPathsReadable(t, fops)
}

func TestStepCrap_NilToolResolver(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner()
	harness, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir, testPackages,
		runner, fops, store, testStepCrapID,
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCrap(
		context.Background(),
		8.0,
		testRunStepID,
		testCrapCommandScope(""),
		testHarnessGocycloSpec,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when ToolResolver is nil")
	}
	if !errors.Is(err, h.ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}
