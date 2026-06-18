// Vision: [MutationRunner] preflight and parity with [MutationSites]/[MutationKills] on fakes.
package gate

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func mustMutationScanStepID(tb testing.TB, out *MutationScanOutput) string {
	tb.Helper()
	s, err := out.StepID()
	if err != nil {
		tb.Fatalf("StepID: %v", err)
	}
	return s
}

func TestNewMutationRunnerScanRejectsNilDependencies(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	fakeResolver := gatetest.NewFakeToolResolver()

	cases := []struct {
		name  string
		runFn func() error
	}{
		{
			"nil runner",
			func() error {
				_, e := NewMutationRunner(nil, fakeResolver, store, mem)
				return e
			},
		},
		{
			"nil resolver",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), nil, store, mem)
				return e
			},
		},
		{
			"nil store",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), fakeResolver, nil, mem)
				return e
			},
		},
		{
			"nil fileOps",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), fakeResolver, store, nil)
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if e := tc.runFn(); e == nil {
				t.Fatal("expected error")
			} else if !errors.Is(e, ErrNilDependency) {
				t.Fatalf("expected ErrNilDependency, got %v", e)
			}
		})
	}
}

func TestNewMutationRunnerKillRejectsNilDependencies(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	fakeResolver := gatetest.NewFakeToolResolver()
	cases := []struct {
		name  string
		runFn func() error
	}{
		{
			"nil runner",
			func() error {
				_, e := NewMutationRunner(nil, fakeResolver, store, mem)
				return e
			},
		},
		{
			"nil resolver",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), nil, store, mem)
				return e
			},
		},
		{
			"nil store",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), fakeResolver, nil, mem)
				return e
			},
		},
		{
			"nil fileOps",
			func() error {
				_, e := NewMutationRunner(noopGoFakeRunner(), fakeResolver, store, nil)
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if e := tc.runFn(); e == nil {
				t.Fatal("expected error")
			} else if !errors.Is(e, ErrNilDependency) {
				t.Fatalf("expected ErrNilDependency, got %v", e)
			}
		})
	}
}

func TestNewMutationRunnerKillDoesNotRequireKillMinRateOption(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go run "+testGremlinsToolSpec, gatetest.Gremlins(mem, fakeRoot, testMutationKillsGremlinsReport)),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	resolver := gatetest.NewFakeToolResolver()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	ctx := context.Background()
	store := NewArtifactStore()
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, fakeRoot, scope)
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	_, err = mr.Kill(ctx, fakeRoot, scope, inv, testGremlinsTool)
	if err != nil {
		if errors.Is(err, ErrInvalidOption) {
			t.Fatalf("unexpected ErrInvalidOption: %v", err)
		}
		t.Fatalf("Kill: %v", err)
	}
}

// compareScanOutputToMutationSites checks [MutationRunner.Scan] stores mutations.json and
// [MutationSites] accepts the same artifact when given the scan token.
func compareScanOutputToMutationSites(
	tb testing.TB,
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	mem FileOps,
	fakeRoot string,
	scope QualityScope,
) {
	tb.Helper()

	store := NewArtifactStore()
	inv := mustQualityScopeInventoryForTests(tb, runner, store, mem, fakeRoot, scope)
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		tb.Fatalf("NewMutationRunner: %v", err)
	}
	scanOut, err := mr.Scan(ctx, fakeRoot, scope, inv, testGremlinsTool)
	if err != nil {
		tb.Fatalf("Scan: %v", err)
	}
	scanID := mustMutationScanStepID(tb, &scanOut)
	if !strings.HasPrefix(scanID, "mutationscan-") {
		tb.Fatalf("MutationRunner.Scan token id want mutationscan- prefix, got %q", scanID)
	}
	if err := MutationSites(scanOut, MaxSites(30)); err != nil {
		tb.Fatalf("MutationSites: %v", err)
	}
}

func TestMutationRunnerScanMatchesMutationSitesArtifactAndScope(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	resolver := gatetest.NewFakeToolResolver()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	compareScanOutputToMutationSites(t, context.Background(), runner, resolver, mem, fakeRoot, scope)
}

func TestMutationRunnerKillMatchesMutationKillsOutput(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go run "+testGremlinsToolSpec, gatetest.Gremlins(mem, fakeRoot, testMutationKillsGremlinsReport)),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	resolver := gatetest.NewFakeToolResolver()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	ctx := context.Background()

	storeKills := NewArtifactStore()
	killsInv := mustQualityScopeInventoryForTests(t, runner, storeKills, mem, fakeRoot, scope)
	killsOut, err := MutationKills(
		ctx, runner, resolver, storeKills, mem, fakeRoot, scope, killsInv, MinKillRate(0), testGremlinsTool,
	)
	if err != nil {
		t.Fatalf("MutationKills: %v", err)
	}
	kKilled, err := killsOut.TotalKilled()
	if err != nil {
		t.Fatalf("TotalKilled: %v", err)
	}

	storeRunner := NewArtifactStore()
	runInv := mustQualityScopeInventoryForTests(t, runner, storeRunner, mem, fakeRoot, scope)
	mr, err := NewMutationRunner(runner, resolver, storeRunner, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	runOut, err := mr.Kill(ctx, fakeRoot, scope, runInv, testGremlinsTool)
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}
	rKilled, err := runOut.TotalKilled()
	if err != nil {
		t.Fatalf("TotalKilled: %v", err)
	}
	if rKilled != kKilled {
		t.Fatalf("TotalKilled: got %d want %d", rKilled, kKilled)
	}
}

// TestMutationKillsReturnsPopulatedOutputOnKillRateFailure verifies bundled MutationKillRate runs after
// the harness producer and preserves a usable kills token when the rate check fails.
func TestMutationKillsReturnsPopulatedOutputOnKillRateFailure(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	mixed := []byte(
		`{"files":[{"file_name":"pkg/m.go","mutations":[{"status":"KILLED"},{"status":"LIVED"}]}]}`,
	)
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go run "+testGremlinsToolSpec, gatetest.Gremlins(mem, fakeRoot, mixed)),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)
	resolver := gatetest.NewFakeToolResolver()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	ctx := context.Background()
	store := NewArtifactStore()
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, fakeRoot, scope)
	out, err := MutationKills(
		ctx,
		runner,
		resolver,
		store,
		mem,
		fakeRoot,
		scope,
		inv,
		MinKillRate(99),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected kill-rate failure")
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
	killed, kErr := out.TotalKilled()
	if kErr != nil {
		t.Fatalf("TotalKilled: %v", kErr)
	}
	if killed != 1 {
		t.Fatalf("TotalKilled on failed token: got %d want 1", killed)
	}
}
