// Vision: MutationKills step: full gremlins run, JSON reports, kill-rate thresholds, and artifacts under fakes.
package harness_test

import (
	"bytes"
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

func stepMutationKillsForTest(tb testing.TB, harness *h.StepRunner, minKillRate int, excludeSegments string) error {
	tb.Helper()
	return harness.StepMutationKills(
		context.Background(),
		minKillRate,
		testMutationCommandScope(tb, excludeSegments),
		testHarnessGremlinsSpec,
		nil,
	)
}

func hasGremlinsFlagInArgs(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func hasUnleashInArgs(args []string) bool {
	for _, a := range args {
		if a == "unleash" {
			return true
		}
	}
	return false
}

func findGremlinsCall(calls []cmdrunner.Command) ([]string, bool) {
	for _, cmd := range calls {
		name := cmd.Name()
		isLocal := isGremlinRunnerCommand(name)
		isGoRun := name == "go" && cmd.Arg(0) == goArgvRun
		if !isLocal && !isGoRun {
			continue
		}
		args := cmd.Args()
		if hasUnleashInArgs(args) {
			return args, true
		}
	}
	return nil, false
}

func assertGremlinsHasNoFlag(tb testing.TB, calls []cmdrunner.Command, flag string) {
	tb.Helper()
	args, found := findGremlinsCall(calls)
	if !found {
		tb.Fatal("expected a gremlins invocation containing unleash")
	}
	if hasGremlinsFlagInArgs(args, flag) {
		tb.Fatalf("gremlins: want NO %s, but found it in %#v", flag, args)
	}
}

func TestStepMutationKills_Success(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 50, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
}

func TestStepMutationKills_GremlinsMutationsOutputPathCanonical(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 50, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
	assertGremlinsMutationsOutIsCanonicalRelative(t, runner.Calls())
}

func TestStepMutationKills_RejectsArtifactSubdirTraversalAtConstruction(t *testing.T) {
	t.Parallel()
	deps := validDeps(cmdtest.NewFakeRunner())
	_, err := h.NewStepRunner(
		testHarnessRoot,
		filepath.Join(testHarnessRoot, "..", "outside-artifacts"),
		testPackages,
		deps.Runner,
		deps.FileOps,
		h.NewDiscardArtifactStore(),
		"",
		deps.Options...,
	)
	if err == nil {
		t.Fatal("expected NewStepRunner to reject traversal artifact subdir before any mutation/go list calls")
	}
	if !errors.Is(err, h.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestStepMutationKills_WalkFailsBeforeGoList(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = newMemFileOpsWalkFail(errSimulatedFailure)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationKillsForTest(t, harness, 50, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
	if len(runner.Calls()) != 0 {
		t.Fatalf("MutationKills must exit before invoking go list when Walk fails first; commands=%v", runner.Calls())
	}
}

func TestStepMutationKills_StoresArtifact(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	stepMutKillID := "mutationkills-step"
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, stepMutKillID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 50, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
	if !store.Has(stepMutKillID, "mutations.json") {
		t.Fatal("expected mutations.json in artifact store after successful mutation kills step")
	}
	prov, ok := store.Provenance(stepMutKillID, "mutations.json")
	if !ok {
		t.Fatal("expected provenance for mutations.json in artifact store")
	}
	if prov.StepID != stepMutKillID {
		t.Errorf("provenance StepID = %q, want %q", prov.StepID, stepMutKillID)
	}
	if prov.Tool != "gremlins run unleash (full)" {
		t.Errorf("provenance Tool = %q, want %q", prov.Tool, "gremlins run unleash (full)")
	}
	if prov.Packages != testPackages {
		t.Errorf("provenance Packages = %q, want %q", prov.Packages, testPackages)
	}
}

func TestStepMutationKills_ParseOnlyIgnoresKillRateAgainstThreshold(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"},{"status":"LIVED"},{"status":"LIVED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 90, ""); err != nil {
		t.Fatalf(
			"harness parses and stores mutations only (kill-rate thresholds are enforced in gate MutationKillRate): %v",
			err,
		)
	}
}

func TestStepMutationKills_MkdirFails(t *testing.T) {
	t.Parallel()
	fops := newMemFileOpsMkdirFail(errSimulatedFailure)
	deps := validDeps(cmdtest.NewFakeRunner())
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationKillsForTest(t, harness, 50, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestStepMutationKills_GremlinsFails(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Fail(errSimulatedFailure)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationKillsForTest(t, harness, 50, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestStepMutationKills_AllPackagesExcluded(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationKillsForTest(t, harness, 50, "example.com/mod/pkg")
	if err == nil {
		t.Fatal("expected error when all packages excluded")
	}
	if !errors.Is(err, h.ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
	if !errors.Is(err, gatecheck.ErrAllPackagesExcluded) {
		t.Fatalf("expected ErrAllPackagesExcluded in chain, got %v", err)
	}
}

func TestStepMutationKills_GremlinsInvokesNoFlag(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 50, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
	assertGremlinsHasNoFlag(t, runner.Calls(), "--dry-run")
}

func TestStepMutationKills_MixedStatus(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	mutationJSON := []byte(
		`{"files":[{"file_name":"pkg/foo.go","mutations":[` +
			`{"status":"KILLED"},{"status":"KILLED"},` +
			`{"status":"KILLED"},{"status":"LIVED"}]}]}`,
	)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, mutationJSON)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	// The harness validates and stores structurally valid gremlins JSON; the gate step owns threshold enforcement.
	if err := stepMutationKillsForTest(t, harness, 70, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
}

func TestStepMutationKills_ParseErrorIncludesRawJSON(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	rawJSON := `{"mutations":{"oops":"not-an-array"}}`
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(rawJSON))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationKillsForTest(t, harness, 50, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
	if !strings.HasSuffix(err.Error(), "\n"+rawJSON) {
		t.Fatalf("expected error chain to quote raw mutations JSON via formatMutationParseFailure")
	}
	stored, readErr := fops.ReadFile(testHarnessMutationsLogicalRel)
	if readErr != nil {
		t.Fatalf("ReadFile mutations.json: %v", readErr)
	}
	if !bytes.Equal(stored, []byte(rawJSON)) {
		t.Fatalf("stored mutations = %q, want %q", stored, rawJSON)
	}
}

func TestStepMutationKills_GoRunFallback(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("go run", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	resolver := gatetest.NewFakeToolResolver()
	harness, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir, testPackages,
		runner, fops, h.NewDiscardArtifactStore(), "",
		h.WithToolResolver(resolver),
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationKillsForTest(t, harness, 50, ""); err != nil {
		t.Fatalf("StepMutationKills: %v", err)
	}
	assertGremlinsMutationsOutIsCanonicalRelative(t, runner.Calls())
}
