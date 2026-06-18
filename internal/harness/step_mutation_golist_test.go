// Vision: StepMutationScan go-list directory normalization cases.
package harness_test

import (
	"context"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

func TestStepMutationScan_GoListPkgDirRootRelativeWithoutModuleColumn(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	pkgDirUnderRoot := testHarnessHostDir(t, "meas", "p")
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("", "",
			map[string]gatetest.PackageListInfo{
				"example.com/mod/meas": {
					Dir:     pkgDirUnderRoot,
					GoFiles: []string{"x.go"},
				},
			})),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"meas/p/x.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, ""), testHarnessGremlinsSpec, nil,
	); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	assertGremlinsHasSingleDryRun(t, runner.Calls())
}

func TestStepMutationScan_GoListModuleDirColumn(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList("example.com/mod", "/mod",
			map[string]gatetest.PackageListInfo{
				"example.com/mod/p": gatetest.DirOnly("/mod/p"),
			})),
		cmdtest.On("gremlins", gatetest.Gremlins(fops, testHarnessRoot, []byte(
			`{"files":[{"file_name":"p/foo.go","mutations":[{"status":"KILLED"}]}]}`,
		))),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, ""), testHarnessGremlinsSpec, nil,
	); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	assertGremlinsHasSingleDryRun(t, runner.Calls())
}

func TestStepMutationScan_GoRunFallback(t *testing.T) {
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
	if err := harness.StepMutationScan(
		context.Background(), 50, testMutationCommandScope(t, ""), testHarnessGremlinsSpec, nil,
	); err != nil {
		t.Fatalf("StepMutationScan: %v", err)
	}
	assertGremlinsMutationsOutIsCanonicalRelative(t, runner.Calls())
}
