// Vision: QualityScope → gremlins argv parity (dry-run vs full-run) and concrete --exclude-files.
package harness_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

func gremlinsExcludeFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--exclude-files=") {
			out = append(out, a)
		}
	}
	return out
}

func mutationUnleashArgs(calls []cmdrunner.Command) ([]string, bool) {
	args, ok := findGremlinsCall(calls)
	if !ok {
		return nil, false
	}
	return args, true
}

func assertArgsContain(t *testing.T, args []string, want string) {
	t.Helper()
	if !slices.Contains(args, want) {
		t.Fatalf("expected %q in %v", want, args)
	}
}

func assertArgsOmit(t *testing.T, args []string, unwanted string) {
	t.Helper()
	if slices.Contains(args, unwanted) {
		t.Fatalf("unexpected %q in %v", unwanted, args)
	}
}

func normalizedMutationScopeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch arg {
		case "--dry-run":
			continue
		case "-o":
			out = append(out, arg, "<mutations.json>")
			if idx+1 < len(args) {
				idx++
			}
		default:
			out = append(out, arg)
		}
	}
	return out
}

func testScopeABInventoryRows() []gatecheck.MutationPackageRow {
	return []gatecheck.MutationPackageRow{
		{ImportPath: "example.com/mod/a", PkgDirRootRel: "a", GoFileNames: []string{"a.go"}},
		{ImportPath: "example.com/mod/b", PkgDirRootRel: "b", GoFileNames: []string{"b.go"}},
	}
}

func TestStepMutationScanAndKills_sameExcludeArgv(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	pkgs := map[string]gatetest.PackageListInfo{
		"example.com/mod/a": {Dir: "/mod/a", GoFiles: []string{"a.go"}},
		"example.com/mod/b": {Dir: "/mod/b", GoFiles: []string{"b.go"}},
	}
	mutJSON := []byte(`{"files":[{"file_name":"a/a.go","mutations":[{"status":"KILLED"}]}]}`)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, mutJSON)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness1, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	scanCommandScope := testQualityScopeCommandScope(testScopeABInventoryRows(), "b", nil, nil)
	if scanErr := harness1.StepMutationScan(
		context.Background(), 50, scanCommandScope, testHarnessGremlinsSpec, nil,
	); scanErr != nil {
		t.Fatalf("StepMutationScan: %v", scanErr)
	}
	scanCalls := runner.Calls()
	scanArgs, ok := mutationUnleashArgs(scanCalls)
	if !ok {
		t.Fatal("expected gremlins unleash after scan")
	}
	scanEx := gremlinsExcludeFlags(scanArgs)

	runner2 := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, mutJSON)),
	)
	deps2 := validDeps(runner2)
	deps2.FileOps = fops
	harness2, err := newTestHarness(testHarnessRoot, testPackages, deps2, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	killCommandScope := testQualityScopeCommandScope(testScopeABInventoryRows(), "b", nil, nil)
	if killErr := harness2.StepMutationKills(
		context.Background(), 0, killCommandScope, testHarnessGremlinsSpec, nil,
	); killErr != nil {
		t.Fatalf("StepMutationKills: %v", killErr)
	}
	killArgs, ok := mutationUnleashArgs(runner2.Calls())
	if !ok {
		t.Fatal("expected gremlins unleash after kills")
	}
	killEx := gremlinsExcludeFlags(killArgs)
	if got, want := normalizedMutationScopeArgs(scanArgs), normalizedMutationScopeArgs(killArgs); !slices.Equal(
		got, want,
	) {
		t.Fatalf("scan/kill gremlins argv differ beyond --dry-run and output path:\nscan %v\nkill %v", got, want)
	}
	if !reflectStringSliceEqual(scanEx, killEx) {
		t.Fatalf("scan excludes %v != kill excludes %v", scanEx, killEx)
	}
	want := []string{"--exclude-files=^b(/|$)"}
	if !reflectStringSliceEqual(scanEx, want) {
		t.Fatalf("scan excludes got %v want %v", scanEx, want)
	}
}

func TestStepMutationScan_mergesQualityAndMutationTags(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	pkgs := map[string]gatetest.PackageListInfo{
		"example.com/mod/pkg": {Dir: "/mod/pkg", GoFiles: []string{"pkg.go"}},
	}
	mutJSON := []byte(`{"files":[{"file_name":"pkg/pkg.go","mutations":[{"status":"KILLED"}]}]}`)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, mutJSON)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	tagCommandScope := testQualityScopeCommandScope(testMutationInventoryRows(t), "", nil, []string{"mage"})
	if scanErr := harness.StepMutationScan(
		context.Background(), 50, tagCommandScope, testHarnessGremlinsSpec,
		[]string{"--tags=integration", "--timeout=1m"},
	); scanErr != nil {
		t.Fatalf("StepMutationScan: %v", scanErr)
	}
	args, ok := mutationUnleashArgs(runner.Calls())
	if !ok {
		t.Fatal("expected gremlins unleash after scan")
	}
	assertArgsContain(t, args, "--tags=mage,integration")
	assertArgsContain(t, args, "--timeout=1m")
	assertArgsOmit(t, args, "--tags=integration")
}

func TestStepMutationScan_excludesSourceFilesOutsideGoListInventoryOnlyBySegment_dotRoot(t *testing.T) {
	t.Parallel()
	assertStepMutationScanExcludesUnlistedTestdata(t, testHarnessArtifactSubdir)
}

// Regression: empty artifactSubdir triggers MkdirTemp + ensureMutationArtifactDir.
// On macOS/Linux the old code resolved the temp dir to an absolute OS path via
// filepath.Abs, then MkdirAll'd it in MemMapFs. That created /Users/… (or /home/…)
// entries under root "/", whose base names didn't match their MemMapFs keys when
// afero.Walk reassembled paths with filepath.Join. Walk aborted on the first
// ErrNotExist before reaching the fixture files, so rootRelativeGoSourceFiles
// silently returned nil and no --exclude-files flags were emitted.
func TestStepMutationScan_excludesUnlistedFilesWithEmptyArtifactSubdir(t *testing.T) {
	t.Parallel()
	assertStepMutationScanExcludesUnlistedTestdata(t, "")
}

func assertStepMutationScanExcludesUnlistedTestdata(t *testing.T, artifactSubdir string) {
	t.Helper()
	fops := gatetest.NewMemoryFileOps()
	if err := fops.Root("."); err != nil {
		t.Fatal(err)
	}
	if err := fops.MkdirAll("magefiles", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fops.WriteFile("magefiles/magefile.go", []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fops.MkdirAll("testdata/failures", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fops.WriteFile("testdata/failures/calc.go", []byte("package failures\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]gatetest.PackageListInfo{
		"example.com/mod/pkg": {Dir: "/mod/pkg", GoFiles: []string{"pkg.go"}},
	}
	mutJSON := []byte(`{"files":[{"file_name":"pkg/pkg.go","mutations":[{"status":"KILLED"}]}]}`)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, ".", mutJSON)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := h.NewStepRunner(
		".",
		artifactSubdir,
		testPackages,
		deps.Runner,
		deps.FileOps,
		h.NewDiscardArtifactStore(),
		"",
		deps.Options...,
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	t.Cleanup(func() { _ = harness.Cleanup() })
	if scanErr := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, "testdata"), testHarnessGremlinsSpec, nil,
	); scanErr != nil {
		t.Fatalf("StepMutationScan: %v", scanErr)
	}
	args, ok := mutationUnleashArgs(runner.Calls())
	if !ok {
		t.Fatal("expected gremlins unleash after scan")
	}
	want := []string{
		"--exclude-files=^testdata(/|$)",
	}
	if got := gremlinsExcludeFlags(args); !reflectStringSliceEqual(got, want) {
		t.Fatalf("exclude flags got %v want %v", got, want)
	}
}

func TestStepMutationScan_excludesSourceFilesOutsideGoListInventoryOnlyBySegment(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	if err := fops.Root(testHarnessRoot); err != nil {
		t.Fatal(err)
	}
	if err := fops.MkdirAll("magefiles", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fops.WriteFile("magefiles/magefile.go", []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fops.MkdirAll("testdata/failures", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fops.WriteFile(
		"testdata/failures/calc.go", []byte("package failures\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]gatetest.PackageListInfo{
		"example.com/mod/pkg": {Dir: "/mod/pkg", GoFiles: []string{"pkg.go"}},
	}
	mutJSON := []byte(`{"files":[{"file_name":"pkg/pkg.go","mutations":[{"status":"KILLED"}]}]}`)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, mutJSON)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if scanErr := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, "testdata"), testHarnessGremlinsSpec, nil,
	); scanErr != nil {
		t.Fatalf("StepMutationScan: %v", scanErr)
	}
	args, ok := mutationUnleashArgs(runner.Calls())
	if !ok {
		t.Fatal("expected gremlins unleash after scan")
	}
	want := []string{
		"--exclude-files=^testdata(/|$)",
	}
	if got := gremlinsExcludeFlags(args); !reflectStringSliceEqual(got, want) {
		t.Fatalf("exclude flags got %v want %v", got, want)
	}
}

func reflectStringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStepMutationScan_allMutationSourcesExcluded(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	pkgs := map[string]gatetest.PackageListInfo{
		"example.com/mod/pkg": {Dir: "/mod/pkg", GoFiles: []string{"x.go"}},
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod", pkgs)),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = stepMutationScanForTest(t, harness, 50, "", []string{"*.go"})
	if err == nil {
		t.Fatal("expected error when all non-test sources excluded by pattern")
	}
	if !errors.Is(err, h.ErrMutationSitesFailed) {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(err, gatecheck.ErrAllPackagesExcluded) {
		t.Fatalf("expected ErrAllPackagesExcluded in chain, got %v", err)
	}
}
