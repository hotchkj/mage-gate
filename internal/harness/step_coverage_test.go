// Vision: Coverage step: merge unit profiles, apply coverpkg filters, and enforce thresholds under fakes.
package harness_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

func lastGoToolCoverCmd(tb testing.TB, runner *cmdtest.FakeRunner) cmdrunner.Command {
	tb.Helper()
	var last cmdrunner.Command
	found := false
	for _, c := range runner.Calls() {
		if c.Name() == "go" && c.Arg(0) == "tool" && c.Arg(1) == "cover" {
			last = c
			found = true
		}
	}
	if !found {
		tb.Fatal("expected go tool cover invocation")
	}
	return last
}

func canonicalCoverFuncArg(tb testing.TB, cmd cmdrunner.Command) string {
	tb.Helper()
	for _, a := range cmd.Args() {
		if strings.HasPrefix(a, "-func=") {
			return strings.TrimPrefix(a, "-func=")
		}
	}
	tb.Fatalf("missing -func= in go tool cover argv: %v", cmd.Args())
	return ""
}

func assertGoToolCoverFuncPathCanonical(tb testing.TB, cmd cmdrunner.Command, want string) {
	tb.Helper()
	got := filepath.ToSlash(canonicalCoverFuncArg(tb, cmd))
	if filepath.IsAbs(got) {
		tb.Fatalf("go tool cover -func must pass cwd-relative canonical path; got absolute %q", got)
	}
	if got != filepath.ToSlash(want) {
		tb.Fatalf("-func path=%q, want %q", got, want)
	}
}

func writeCoverageArtifact(t *testing.T, store *memStore, profile string) {
	t.Helper()
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, []byte(profile), h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
}

func assertStoredCoverageProfile(t *testing.T, store *memStore, want string) {
	t.Helper()
	stored, readErr := store.Read(testStepCovID, testStoreArtifactCoverage)
	if readErr != nil {
		t.Fatalf("Read stored coverage: %v", readErr)
	}
	if string(stored) != want {
		t.Fatalf("coverage step should persist quality-scoped profile, got %q want %q", string(stored), want)
	}
}

func TestStepCoverage_Pass(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\nsome/pkg/file.go:1.2,3.4 1 1\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(cmdtest.On("go tool cover", gatetest.GoToolCover(100.0)))
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCoverage(
		context.Background(), 90.0, testRunStepID, testEmptyQualityScopeCommandScope(),
	)
	if err != nil {
		t.Fatalf("StepCoverage: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
}

func TestStepCoverage_Fail(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	// 1 covered out of 2 = 50% (below 90% threshold)
	prof := []byte("mode: set\nsome/pkg/file.go:1.2,3.4 1 1\nsome/pkg/file.go:5.0,6.0 1 0\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner(cmdtest.On("go tool cover", gatetest.GoToolCover(50.0)))
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCoverage(context.Background(), 90.0, testRunStepID, testEmptyQualityScopeCommandScope())
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageLogicalRel)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrCoverageFailed) {
		t.Fatalf("expected ErrCoverageFailed, got %v", err)
	}
	var covFail *h.CoverageFailure
	if !errors.As(err, &covFail) {
		t.Fatalf("expected CoverageFailure, got %T: %v", err, err)
	}
	result := covFail.Result()
	if result.TotalCoverage != 50.0 {
		t.Fatalf("coverage result total = %.1f, want 50.0", result.TotalCoverage)
	}
	if len(result.WorstFileRows) != 1 {
		t.Fatalf("worst file rows = %#v, want one row", result.WorstFileRows)
	}
}

func TestStepCoverage_FilteredPass(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	writeCoverageArtifact(t, store, testCoverageMixedVendor)
	runner := cmdtest.NewFakeRunner(cmdtest.On("go tool cover", gatetest.GoToolCover(100.0)))
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	commandScope := testQualityScopeCommandScope(nil, testCoverExcludeVendor, nil, nil)
	if err := harness.StepCoverage(context.Background(), 90.0, testRunStepID, commandScope); err != nil {
		t.Fatalf("StepCoverage: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageFilteredLogicalRel)
	wantStored := "mode: set\ngithub.com/hotchkj/mage-gate/internal/harness/config.go:1.2,3.4 1 1\n"
	assertStoredCoverageProfile(t, store, wantStored)
}

func TestStepCoverage_TestFilePatternsFilterStoredProfile(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := "mode: set\n" +
		"github.com/hotchkj/mage-gate/gate/keep.go:1.2,3.4 1 1\n" +
		"github.com/hotchkj/mage-gate/gate/keep_test.go:1.2,3.4 1 0\n"
	writeCoverageArtifact(t, store, prof)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go tool cover", gatetest.GoToolCover(95.0)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	commandScope := testQualityScopeCommandScope(nil, "", []string{"*_test.go"}, nil)
	if err := harness.StepCoverage(context.Background(), 90.0, testRunStepID, commandScope); err != nil {
		t.Fatalf("StepCoverage: %v", err)
	}
	assertGoToolCoverFuncPathCanonical(t, lastGoToolCoverCmd(t, runner), testHarnessCoverageFilteredLogicalRel)
	wantStored := "mode: set\ngithub.com/hotchkj/mage-gate/gate/keep.go:1.2,3.4 1 1\n"
	assertStoredCoverageProfile(t, store, wantStored)
}

func TestStepCoverage_AllPackagesExcluded(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	prof := []byte("mode: set\ngithub.com/foo/vendor/x.go:1.2,3.4 1 1\n")
	if err := store.Write(testRunStepID, testStoreArtifactCoverage, prof, h.Provenance{}); err != nil {
		t.Fatalf(testFmtStoreWrite, err)
	}
	runner := cmdtest.NewFakeRunner()
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCoverage(
		context.Background(),
		90.0,
		testRunStepID,
		testQualityScopeCommandScope(nil, "vendor", nil, nil),
	)
	if err == nil {
		t.Fatal("expected error when all packages excluded")
	}
	if !errors.Is(err, h.ErrCoverageFailed) {
		t.Fatalf("expected ErrCoverageFailed, got %v", err)
	}
	if !errors.Is(err, gatecheck.ErrAllPackagesExcluded) {
		t.Fatalf("expected ErrAllPackagesExcluded wrapped in error, got %v", err)
	}
	for _, c := range runner.Calls() {
		if c.Name() == "go" && c.Arg(0) == "tool" && c.Arg(1) == "cover" {
			t.Fatalf("go tool cover must not run when all packages are excluded; calls=%v", runner.Calls())
		}
	}
}

func TestStepCoverage_StoreMissingArtifact(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go tool cover", gatetest.GoToolCover(95.0)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCoverage(context.Background(), 90.0, testRunStepID, testEmptyQualityScopeCommandScope())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrCoverageFailed) {
		t.Fatalf("expected ErrCoverageFailed, got %v", err)
	}
}

func TestStepCoverage_RejectsMissingUpstreamStepID(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go tool cover", gatetest.GoToolCover(95.0)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepCovID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepCoverage(context.Background(), 90.0, "", testEmptyQualityScopeCommandScope())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrCoverageFailed) {
		t.Fatalf("expected ErrCoverageFailed, got %v", err)
	}
}
