// Vision: Lint step: golangci-lint command construction, resolver probes, and stderr-shaped failures under fakes.
package harness_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

const testLintToolSpec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

func findGoCustomLintBuild(tb testing.TB, calls []cmdrunner.Command) cmdrunner.Command {
	tb.Helper()
	for _, cmd := range calls {
		if cmd.Name() != "go" || cmd.Arg(0) != "run" {
			continue
		}
		args := cmd.Args()
		if slices.Contains(args, "custom") && slices.Contains(args, "--destination") {
			return cmd
		}
	}
	tb.Fatal("expected go run ... custom ... --destination invocation")
	return cmdrunner.Command{}
}

func TestStepLint_Success(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepLint(context.Background(), testLintConfig, "", "", testLintToolSpec, nil); err != nil {
		t.Fatalf("StepLint: %v", err)
	}
	calls := runner.Calls()
	var lintCall cmdrunner.Command
	for _, c := range calls {
		if c.Name() == "golangci-lint" && slices.Contains(c.Args(), "run") {
			lintCall = c
			break
		}
	}
	if lintCall.Name() == "" {
		t.Fatalf("expected golangci-lint run call; got %v", calls)
	}
	assertFlagFollowedBy(t, lintCall.Args(), "-c", testLintConfig)
}

func TestStepLint_AppendsLintArgsAfterBasePrefix(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepLint(
		context.Background(),
		testLintConfig,
		"",
		"",
		testLintToolSpec,
		[]string{"--verbose", "--max-issues-per-linter=0"},
	); err != nil {
		t.Fatalf("StepLint: %v", err)
	}
	var lintCall cmdrunner.Command
	found := false
	for _, c := range runner.Calls() {
		if c.Name() == "golangci-lint" {
			lintCall = c
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected golangci-lint call")
	}
	args := lintCall.Args()
	assertFlagFollowedBy(t, args, "-c", testLintConfig)
	wantSuffix := []string{"--verbose", "--max-issues-per-linter=0"}
	if len(args) < len(wantSuffix) {
		t.Fatalf("golangci-lint args too short: %v", args)
	}
	gotSuffix := args[len(args)-len(wantSuffix):]
	if !slices.Equal(gotSuffix, wantSuffix) {
		t.Fatalf("golangci-lint suffix = %v, want %v (full args: %v)", gotSuffix, wantSuffix, args)
	}
}

func TestStepLint_WithCustomGCL(t *testing.T) {
	t.Parallel()
	customGCLPath := "/test-root/custom/.custom-gcl.yml"
	wantDestNative, resolveErr := h.ResolveWithinRoot(testHarnessRoot, testHarnessArtifactSubdir)
	if resolveErr != nil {
		t.Fatalf("ResolveWithinRoot artifact host projection: %v", resolveErr)
	}
	customLintKey := testHarnessCustomLintExeLogicalCmdKey()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go run", gatetest.NoopCommand),
		cmdtest.On(customLintKey, gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepLint(
		context.Background(),
		testLintConfig,
		customGCLPath,
		testLintToolSpec,
		testLintToolSpec,
		nil,
	); err != nil {
		t.Fatalf("StepLint: %v", err)
	}
	customBuild := findGoCustomLintBuild(t, runner.Calls())
	wantCustomBuildCwd := cmdrunner.DirNativeForExec(filepath.Dir(customGCLPath))
	if got := customBuild.Dir(); got != wantCustomBuildCwd {
		t.Fatalf(
			"custom build cwd: Dir()=%q want %q (DirNativeForExec(filepath.Dir(customGCLPath)))",
			got,
			wantCustomBuildCwd,
		)
	}
	assertFlagFollowedBy(t, customBuild.Args(), "--destination", wantDestNative)
	var customLintExec cmdrunner.Command
	for _, c := range runner.Calls() {
		if c.Name() == customLintKey {
			customLintExec = c
			break
		}
	}
	if customLintExec.Name() == "" {
		t.Fatalf("expected custom linter invocation with name %q; calls=%v", customLintKey, runner.Calls())
	}
	assertFlagFollowedBy(t, customLintExec.Args(), "-c", testLintConfig)
}

func TestStepLint_WithCustomGCLRequiresCustomLintSpec(t *testing.T) {
	t.Parallel()
	customGCLPath := "/test-root/custom/.custom-gcl.yml"
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go run", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepLint(
		context.Background(),
		testLintConfig,
		customGCLPath,
		"",
		testLintToolSpec,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when custom GCL has no custom lint spec")
	}
	if !errors.Is(err, h.ErrLintFailed) {
		t.Fatalf("expected ErrLintFailed, got %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 0 {
		t.Fatalf("expected no commands to run, got %v", calls)
	}
}

func TestStepLint_Failure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", gatetest.Fail(errSimulatedFailure)),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepLint(context.Background(), testLintConfig, "", "", testLintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrLintFailed) {
		t.Fatalf("expected ErrLintFailed, got %v", err)
	}
}

func TestStepLint_ArtifactDirFails(t *testing.T) {
	t.Parallel()
	fops := newMemFileOpsMkdirFail(errSimulatedFailure)
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepLint(context.Background(), testLintConfig, "", "", testLintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrLintFailed) {
		t.Fatalf("expected ErrLintFailed, got %v", err)
	}
}

func TestStepLint_EmptyLintConfigPath(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepLint(context.Background(), "", "", "", testLintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error when LintConfigPath is empty")
	}
	if !errors.Is(err, h.ErrLintFailed) {
		t.Fatalf("expected ErrLintFailed, got %v", err)
	}
}
