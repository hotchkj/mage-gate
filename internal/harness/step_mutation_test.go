// Vision: StepMutationScan (dry-run): gremlins JSON, in-harness site caps, store wiring under fakes.
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

const testHarnessGremlinsSpec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

func stepMutationScanForTest(
	tb testing.TB,
	harness *h.StepRunner,
	maxSites int,
	excludeSegments string,
	testFilePatterns []string,
) error {
	tb.Helper()
	commandScope := testQualityScopeCommandScope(testMutationInventoryRows(tb), excludeSegments, testFilePatterns, nil)
	return harness.StepMutationScan(
		context.Background(),
		maxSites,
		commandScope,
		testHarnessGremlinsSpec,
		nil,
	)
}

// isGremlinRunnerCommand reports whether cmd.Name() refers to a gremlins binary
// (local path or bare name), without using strings.Contains (forbidigo in tests).
func isGremlinRunnerCommand(name string) bool {
	if name == "gremlins" || strings.EqualFold(name, "gremlins.exe") {
		return true
	}
	lower := strings.ToLower(name)
	return strings.HasSuffix(name, "/gremlins") ||
		strings.HasSuffix(lower, `\gremlins.exe`) ||
		strings.HasSuffix(lower, `/gremlins.exe`)
}

func isGremlinsUnleash(cmd cmdrunner.Command) bool {
	name := cmd.Name()
	isLocal := isGremlinRunnerCommand(name)
	isGoRun := name == "go" && cmd.Arg(0) == "run"
	if !isLocal && !isGoRun {
		return false
	}
	for _, a := range cmd.Args() {
		if a == "unleash" {
			return true
		}
	}
	return false
}

func countArgOccurrences(args []string, want string) int {
	count := 0
	for _, a := range args {
		if a == want {
			count++
		}
	}
	return count
}

func indexArg(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

func assertGremlinsMutationsOutIsCanonicalRelative(tb testing.TB, calls []cmdrunner.Command) {
	tb.Helper()
	found := false
	for _, cmd := range calls {
		if !isGremlinsUnleash(cmd) {
			continue
		}
		found = true
		args := cmd.Args()
		oIdx := indexArg(args, "-o")
		if oIdx == -1 || oIdx+1 >= len(args) {
			tb.Fatalf("-o missing path operand in %#v", args)
		}
		got := args[oIdx+1]
		if filepath.IsAbs(got) {
			tb.Fatalf("-o path must not be an absolute host path: %q", got)
		}
		if filepath.VolumeName(got) != "" {
			tb.Fatalf("-o path must not use a volume or drive prefix: %q", got)
		}
		if got != testHarnessMutationsLogicalRel {
			tb.Fatalf("-o path got %q, want root-relative canonical %q", got, testHarnessMutationsLogicalRel)
		}
	}
	if !found {
		tb.Fatal("expected gremlins unleash")
	}
}

func assertGremlinsHasSingleDryRun(tb testing.TB, calls []cmdrunner.Command) {
	tb.Helper()
	found := false
	for _, cmd := range calls {
		if !isGremlinsUnleash(cmd) {
			continue
		}
		found = true
		args := cmd.Args()
		if dry := countArgOccurrences(args, "--dry-run"); dry < 1 {
			tb.Fatalf("gremlins: expected at least one --dry-run, got %d in %#v", dry, args)
		}
	}
	if !found {
		tb.Fatal("expected a gremlins invocation containing unleash")
	}
}

func TestStepMutationScan_Success(t *testing.T) {
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
	if err := stepMutationScanForTest(t, harness, 50, "", nil); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
}

func TestStepMutationScan_GremlinsMutationsOutputPathCanonical(t *testing.T) {
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
	if err := stepMutationScanForTest(t, harness, 50, "", nil); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	assertGremlinsMutationsOutIsCanonicalRelative(t, runner.Calls())
}

func TestStepMutationScan_StoresArtifact(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	stepMutID := "mutationsites-step"
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
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, stepMutID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := stepMutationScanForTest(t, harness, 50, "", nil); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	if !store.Has(stepMutID, "mutations.json") {
		t.Fatal("expected mutations.json in artifact store after successful mutation step")
	}
	prov, ok := store.Provenance(stepMutID, "mutations.json")
	if !ok {
		t.Fatal("expected provenance for mutations.json in artifact store")
	}
	if prov.StepID != stepMutID {
		t.Errorf("provenance StepID = %q, want %q", prov.StepID, stepMutID)
	}
	if prov.Tool != "gremlins run unleash" {
		t.Errorf("provenance Tool = %q, want %q", prov.Tool, "gremlins run unleash")
	}
	if prov.Packages != testPackages {
		t.Errorf("provenance Packages = %q, want %q", prov.Packages, testPackages)
	}
}

func TestStepMutationScan_AppendsMutationArgsAfterDryRun(t *testing.T) {
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
	if err := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, ""), testHarnessGremlinsSpec, []string{"--timeout=1m"},
	); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	found := false
	for _, cmd := range runner.Calls() {
		if !isGremlinsUnleash(cmd) {
			continue
		}
		found = true
		args := cmd.Args()
		dryIdx := indexArg(args, "--dry-run")
		timeoutIdx := indexArg(args, "--timeout=1m")
		if dryIdx == -1 || timeoutIdx == -1 {
			t.Fatalf("expected --dry-run and --timeout=1m in %v", args)
		}
		if dryIdx >= timeoutIdx {
			t.Fatalf("expected --dry-run before passthrough args in %v", args)
		}
	}
	if !found {
		t.Fatal("expected gremlins unleash call")
	}
}

func TestStepMutationScan_FailThreshold(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"},{"status":"LIVED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationScanForTest(t, harness, 1, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestStepMutationScan_WalkFailsBeforeGoList(t *testing.T) {
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
	err = stepMutationScanForTest(t, harness, 50, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
	if len(runner.Calls()) != 0 {
		t.Fatalf("MutationScan must exit before invoking go list when Walk fails first; commands=%v", runner.Calls())
	}
}

func TestStepMutationScan_MkdirFails(t *testing.T) {
	t.Parallel()
	fops := newMemFileOpsMkdirFail(errSimulatedFailure)
	deps := validDeps(cmdtest.NewFakeRunner())
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationScanForTest(t, harness, 50, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestStepMutationScan_RejectsArtifactSubdirTraversal(t *testing.T) {
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
		t.Fatal("expected NewStepRunner to reject traversal artifact subdir")
	}
	if !errors.Is(err, h.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestStepMutationScan_GremlinsFails(t *testing.T) {
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
	err = stepMutationScanForTest(t, harness, 50, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestStepMutationScan_AllPackagesExcluded(t *testing.T) {
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
	err = stepMutationScanForTest(t, harness, 50, "example.com/mod/pkg", nil)
	if err == nil {
		t.Fatal("expected error when all packages excluded")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
	if !errors.Is(err, gatecheck.ErrAllPackagesExcluded) {
		t.Fatalf("expected ErrAllPackagesExcluded in chain, got %v", err)
	}
}

func TestStepMutationScan_ExcessSitesPerFile(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	var sb strings.Builder
	sb.WriteString(`{"files":[{"file_name":"pkg/foo.go","mutations":[`)
	for i := 0; i < 51; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString("{}")
	}
	sb.WriteString(`]}]}`)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(sb.String()))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationScanForTest(t, harness, 50, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestStepMutationScan_ParseErrorIncludesRawJSON(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	rawJSON := `{"files":{"oops":"not-an-array"}}`
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
	err = stepMutationScanForTest(t, harness, 50, "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
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

func TestStepMutationScan_GremlinsInvokesDryRunOnce(t *testing.T) {
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
	if err := stepMutationScanForTest(t, harness, 50, "", nil); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	assertGremlinsHasSingleDryRun(t, runner.Calls())
}
