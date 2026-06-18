//go:build mage
// +build mage

package main

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

// Deterministic fake-module constants aligned with [gate/acceptance_helpers_test.go] fakes.
const (
	mutationPhaseFakeModule     = "github.com/hotchkj/mage-gate"
	mutationPhaseFakeRoot       = "/fake-root"
	mutationPhaseFakeGatePkg    = "github.com/hotchkj/mage-gate/gate"
	mutationPhaseFakeHarnessPkg = "github.com/hotchkj/mage-gate/internal/harness"
	mutationPhaseGremlinsSpec   = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
)

// Dry-run report with no mutation sites: harness validation + consumer checks pass with a high site cap.
var mutationPhaseReportSitesOnly = []byte(
	`{"files":[{"file_name":"pkg/foo.go","mutations":[]}]}`,
)

// Same per-file / status mix as [gate.TestMutationCoverage_ThresholdOnStoredReport] (2 RUNNABLE, 1 NOT_COVERED).
var mutationPhaseReportSitesAndCoverage = []byte(`{"mutations":[
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"NOT_COVERED"}
	]}`)

type countingMutationRunner struct {
	inner qg.MutationRunner
	n     *int
}

//nolint:gocritic // Must match the public qg.MutationRunner interface.
func (c *countingMutationRunner) Scan(
	ctx context.Context,
	root string,
	qualityScope qg.QualityScope,
	inventory qg.QualityScopeInventoryOutput,
	gremlinsSpec qg.GremlinsToolValue,
	opts ...qg.MutationOption,
) (qg.MutationScanOutput, error) {
	*c.n++
	return c.inner.Scan(ctx, root, qualityScope, inventory, gremlinsSpec, opts...)
}

//nolint:gocritic // Must match the public qg.MutationRunner interface.
func (c *countingMutationRunner) Kill(
	ctx context.Context,
	root string,
	qualityScope qg.QualityScope,
	inventory qg.QualityScopeInventoryOutput,
	gremlinsSpec qg.GremlinsToolValue,
	opts ...qg.MutationOption,
) (qg.MutationKillsOutput, error) {
	return c.inner.Kill(ctx, root, qualityScope, inventory, gremlinsSpec, opts...)
}

func mutationPhaseFakeMemRunner(t *testing.T, mem *gatetest.MemoryFileOps, report []byte) qg.CommandRunner {
	t.Helper()
	root := mutationPhaseFakeRoot
	inner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(mutationPhaseFakeModule, root, map[string]gatetest.PackageListInfo{
			mutationPhaseFakeGatePkg:    gatetest.DirOnly(filepath.Join(root, "gate")),
			mutationPhaseFakeHarnessPkg: gatetest.DirOnly(filepath.Join(root, "internal", "harness")),
		})),
		cmdtest.On("go run", gatetest.Gremlins(mem, root, report)),
	)
	r, err := qg.NewDisplayRunner(inner, qg.OutputModeAgent, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("NewDisplayRunner: %v", err)
	}
	return r
}

// MutationPhase tests override package-level hook variables; do not use t.Parallel here—concurrent
// tests would race on those globals. resetMutationPhaseHooks registers t.Cleanup to restore defaults.
func resetMutationPhaseHooks(t *testing.T) {
	t.Helper()
	origNew := newGateMutationRunner
	origSites := gateMutationSitesCheck
	origCov := gateMutationCoverageCheck
	t.Cleanup(func() {
		newGateMutationRunner = origNew
		gateMutationSitesCheck = origSites
		gateMutationCoverageCheck = origCov
	})
}

// TestRunGateMutationPhase_OneScan_MutationSitesOnlyWhenCoverageOff proves a single
// [MutationRunner.Scan] and that [MutationCoverage] is not invoked when mutation coverage is disabled.
func TestRunGateMutationPhase_OneScan_MutationSitesOnlyWhenCoverageOff(t *testing.T) {
	resetMutationPhaseHooks(t)
	var scanCount, sitesCalls, coverageCalls int
	newGateMutationRunner = func(
		runner qg.CommandRunner, resolver qg.ToolResolver, store *qg.ArtifactStore, fileOps qg.FileOps,
	) (qg.MutationRunner, error) {
		inner, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
		if err != nil {
			return nil, err
		}
		return &countingMutationRunner{
			inner: inner,
			n:     &scanCount,
		}, nil
	}
	gateMutationSitesCheck = func(
		scanOut qg.MutationScanOutput,
		th qg.MutationSitesThreshold,
	) error {
		sitesCalls++
		return qg.MutationSites(scanOut, th)
	}
	gateMutationCoverageCheck = func(
		scanOut qg.MutationScanOutput,
		th qg.MutationCoverageThreshold,
	) error {
		coverageCalls++
		return qg.MutationCoverage(scanOut, th)
	}

	mem := gatetest.NewMemoryFileOps()
	runner := mutationPhaseFakeMemRunner(t, mem, mutationPhaseReportSitesOnly)
	resolver := gatetest.NewFakeToolResolver()
	store := qg.NewArtifactStore()
	scope, err := qg.NewQualityScope("./...", qualityScopeOptions(&config{})...)
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	sitesMax := 50
	cfg := &config{
		Gremlins:   gremlinsConfig{ToolSpec: mutationPhaseGremlinsSpec},
		Thresholds: thresholdConfig{MutationSitesMax: &sitesMax},
	}
	ctx := context.Background()
	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, mem, mutationPhaseFakeRoot, scope)
	if err != nil {
		t.Fatalf("QualityScopeInventory: %v", err)
	}
	if err := runGateMutationPhase(
		ctx, runner, resolver, store, mem, mutationPhaseFakeRoot, scope, &inv, cfg,
	); err != nil {
		t.Fatalf("runGateMutationPhase: %v", err)
	}
	if scanCount != 1 {
		t.Fatalf("Scan calls: got %d, want 1", scanCount)
	}
	if sitesCalls != 1 {
		t.Fatalf("MutationSites calls: got %d, want 1", sitesCalls)
	}
	if coverageCalls != 0 {
		t.Fatalf("MutationCoverage calls: got %d, want 0 (coverage step disabled)", coverageCalls)
	}
}

// mutationPhaseCounters tracks hook call counts and IDs for the mutation phase test.
type mutationPhaseCounters struct {
	scanCount, sitesCalls, coverageCalls int
	sitesStepID                          string
}

// installMutationPhaseHooks sets up test hooks to count calls and capture step IDs.
func installMutationPhaseHooks(t *testing.T, counters *mutationPhaseCounters) {
	newGateMutationRunner = func(
		runner qg.CommandRunner, resolver qg.ToolResolver, store *qg.ArtifactStore, fileOps qg.FileOps,
	) (qg.MutationRunner, error) {
		inner, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
		if err != nil {
			return nil, err
		}
		return &countingMutationRunner{inner: inner, n: &counters.scanCount}, nil
	}
	gateMutationSitesCheck = func(scanOut qg.MutationScanOutput, th qg.MutationSitesThreshold) error {
		counters.sitesCalls++
		id, err := scanOut.StepID()
		if err != nil {
			return err
		}
		counters.sitesStepID = id
		return qg.MutationSites(scanOut, th)
	}
	gateMutationCoverageCheck = func(scanOut qg.MutationScanOutput, th qg.MutationCoverageThreshold) error {
		counters.coverageCalls++
		id, err := scanOut.StepID()
		if err != nil {
			return err
		}
		if id != counters.sitesStepID {
			t.Errorf("MutationCoverage token stepID: got %q, want %q (same as MutationSites)", id, counters.sitesStepID)
		}
		return qg.MutationCoverage(scanOut, th)
	}
}

// assertMutationPhaseCounters verifies the expected call counts after mutation phase.
func assertMutationPhaseCounters(t *testing.T, counters *mutationPhaseCounters) {
	t.Helper()
	if counters.scanCount != 1 {
		t.Fatalf("Scan calls: got %d, want 1", counters.scanCount)
	}
	if counters.sitesCalls != 1 {
		t.Fatalf("MutationSites calls: got %d, want 1", counters.sitesCalls)
	}
	if counters.coverageCalls != 1 {
		t.Fatalf("MutationCoverage calls: got %d, want 1", counters.coverageCalls)
	}
}

// TestRunGateMutationPhase_OneScan_SameScanOut_MutationSitesAndCoverageWhenCoverageOn proves
// one [MutationRunner.Scan] feeds both [MutationSites] and [MutationCoverage] (same [qg.MutationScanOutput] id).
func TestRunGateMutationPhase_OneScan_SameScanOut_MutationSitesAndCoverageWhenCoverageOn(t *testing.T) {
	resetMutationPhaseHooks(t)
	var counters mutationPhaseCounters
	installMutationPhaseHooks(t, &counters)

	mem := gatetest.NewMemoryFileOps()
	runner := mutationPhaseFakeMemRunner(t, mem, mutationPhaseReportSitesAndCoverage)
	resolver := gatetest.NewFakeToolResolver()
	store := qg.NewArtifactStore()
	scope, err := qg.NewQualityScope("./...", qualityScopeOptions(&config{})...)
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	sitesMax := 50
	covMin := 50
	cfg := &config{
		Gremlins: gremlinsConfig{ToolSpec: mutationPhaseGremlinsSpec},
		Thresholds: thresholdConfig{
			MutationSitesMax:    &sitesMax,
			MutationCoverageMin: &covMin,
		},
	}
	ctx := context.Background()
	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, mem, mutationPhaseFakeRoot, scope)
	if err != nil {
		t.Fatalf("QualityScopeInventory: %v", err)
	}
	if err := runGateMutationPhase(
		ctx, runner, resolver, store, mem, mutationPhaseFakeRoot, scope, &inv, cfg,
	); err != nil {
		t.Fatalf("runGateMutationPhase: %v", err)
	}
	assertMutationPhaseCounters(t, &counters)
}
