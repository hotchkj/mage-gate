// Vision: Output routing and multi-suite progress: silent vs verbose display streams across combined steps.
package gate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

const compileStepOutputLine = "Compile..."

func assertStartLinesExact(t *testing.T, got string, wantLines []string) {
	t.Helper()
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("expected %d start lines, got %d: %q", len(wantLines), len(gotLines), got)
	}
	for idx, want := range wantLines {
		if gotLines[idx] != want {
			t.Fatalf("line %d: expected %q, got %q", idx, want, gotLines[idx])
		}
	}
}

func TestLocalSuccessAgentMode(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &displayErr)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != compileStepOutputLine {
		t.Fatalf("expected compile start line, got %q", displayOut.String())
	}
	if displayErr.Len() != 0 {
		t.Fatalf("silent display leaked stderr to display: %q", displayErr.String())
	}
}

func TestLocalOutputModeAgentContract(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &displayErr)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	if got := RunnerOutputMode(runner); got != OutputModeAgent {
		t.Fatalf("expected runner output mode to be agent, got %q", got)
	}

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != compileStepOutputLine {
		t.Fatalf("expected compile start line in agent mode, got %q", displayOut.String())
	}
	if displayErr.Len() != 0 {
		t.Fatalf("agent mode should not emit subprocess output on success: %q", displayErr.String())
	}
}

func TestLocalFailureDiagnostics(t *testing.T) {
	ctx := context.Background()
	failInner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", gatetest.FailWith(errForcedFailure,
			"# example.com/mod/pkg\npkg/foo.go:5:3: undefined: bar\n")),
		cmdtest.On("golangci-lint", gatetest.FailWith(errForcedFailure,
			"pkg/foo.go:10:5: unused variable 'x' (deadcode)\n")),
	)
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, failInner, OutputModeAgent, &displayOut, &displayErr)
	fileOps := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope)
	if err == nil {
		t.Fatal("expected compile step to fail")
	}
	if got := strings.TrimSpace(displayOut.String()); got != compileStepOutputLine {
		t.Fatalf("expected compile start line on failure, got %q", displayOut.String())
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if got, want := displayErr.String(), de.Error()+"\n"; got != want {
		t.Fatalf("expected exact diagnostic stderr display\\nwant: %q\\ngot:  %q", want, got)
	}
	if de.Name() != "compile" {
		t.Errorf("expected name compile, got %q", de.Name())
	}
}

func TestVerboseModePassesThrough(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := "/fake-test-root"
	inner := acceptanceFakeRunnerForRoot(mem, fakeRoot, cmdtest.On("go build", gatetest.NoopCommand))
	wrapped := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)

	ctx := context.Background()
	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, wrapped, mem, fakeRoot, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Fatal("expected go build to be called")
	}
}

func TestDeadcodeVerboseModeSuccessPasses(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := "/fake-test-root"
	inner := acceptanceFakeRunnerForRoot(mem, fakeRoot, cmdtest.On("deadcode", gatetest.NoopCommand))
	wrapped := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)

	ctx := context.Background()
	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.31.0", true)

	err := Deadcode(
		ctx,
		wrapped,
		fakeResolver,
		mem,
		fakeRoot,
		pkgScope,
		DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
	)
	if err != nil {
		t.Fatalf("Deadcode() failed: %v", err)
	}

	foundDeadcode := false
	for _, cmd := range inner.Calls() {
		if cmd.Name() == cmdDeadcode {
			foundDeadcode = true
			break
		}
	}
	if !foundDeadcode {
		t.Fatal("expected deadcode to be called")
	}
}

func TestVerboseModeFailureReturnsRawErrorWithoutDiagnosticError(t *testing.T) {
	ctx := context.Background()
	failInner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", gatetest.FailWith(errForcedFailure,
			"# example.com/mod/pkg\npkg/foo.go:5:3: undefined: bar\n")),
		cmdtest.On("golangci-lint", gatetest.FailWith(errForcedFailure,
			"pkg/foo.go:10:5: unused variable 'x' (deadcode)\n")),
	)
	runner := mustNewDisplayRunner(t, failInner, OutputModeVerbose, io.Discard, io.Discard)
	fileOps := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot
	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope)
	if err == nil {
		t.Fatal("expected compile step to fail")
	}
	var de *DiagnosticError
	if errors.As(err, &de) {
		t.Fatalf("verbose display must not wrap with *DiagnosticError, got: %v", err)
	}
	if !errors.Is(err, ErrCompileFailed) {
		t.Errorf("expected errors.Is(err, ErrCompileFailed), got: %v", err)
	}
}

func TestMultiSuiteComposition(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem)
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &displayErr)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	scope1 := mustNewQualityScope(t, "./gate/...")
	scope2 := mustNewQualityScope(t, "./internal/...")
	pkgScope1 := mustNewPackageScope(t, "./gate/...")
	pkgScope2 := mustNewPackageScope(t, "./internal/...")
	inv1 := mustQualityScopeInventoryForTests(t, runner, store, fileOps, root, scope1)
	inv2 := mustQualityScopeInventoryForTests(t, runner, store, fileOps, root, scope2)

	unitCov1, err := CoveredTest(ctx, runner, store, fileOps, root, pkgScope1, scope1, inv1)
	if err != nil {
		t.Fatalf("CoveredTest(scope1) failed: %v", err)
	}
	unitCov2, err := CoveredTest(ctx, runner, store, fileOps, root, pkgScope2, scope2, inv2)
	if err != nil {
		t.Fatalf("CoveredTest(scope2) failed: %v", err)
	}

	err = Duration(ctx, runner, store, fileOps, root, mustTestOutputFromCovered(t, &unitCov1), MaxSeconds(1.0))
	if err != nil {
		t.Fatalf("Duration(suite1) failed: %v", err)
	}
	err = Duration(ctx, runner, store, fileOps, root, mustTestOutputFromCovered(t, &unitCov2), MaxSeconds(1.0))
	if err != nil {
		t.Fatalf("Duration(suite2) failed: %v", err)
	}

	covToken := mustCoveredTestOutput(t, &unitCov1)
	covOut, err := Coverage(ctx, runner, store, fileOps, root, covToken, MinPercent(90))
	if err != nil {
		t.Fatalf("Coverage() failed: %v", err)
	}
	err = Crap(
		ctx, runner, gatetest.NewFakeToolResolver(), store, fileOps,
		root, covOut, inv1, MaxScore(8), testGocycloTool,
	)
	if err != nil {
		t.Fatalf("Crap() failed: %v", err)
	}

	if countGoCall(inner.Calls(), cmdTest) != 2 {
		t.Fatalf("expected 2 go test invocations, got %d", countGoCall(inner.Calls(), cmdTest))
	}
	assertStartLinesExact(t, displayOut.String(), []string{
		"Quality Scope Inventory...",
		"Quality Scope Inventory...",
		"Covered Test...",
		"Covered Test...",
		"Duration...",
		"Duration...",
		"Coverage...",
		"CRAP...",
	})
	if displayErr.Len() != 0 {
		t.Fatalf("silent display leaked stderr to display: %q", displayErr.String())
	}
}

func TestDeadcodeSuccessSilent(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("deadcode", gatetest.NoopCommand))
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &displayErr)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.31.0", true)

	err := Deadcode(
		ctx,
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
	)
	if err != nil {
		t.Fatalf("Deadcode() failed: %v", err)
	}
	if got := strings.TrimSpace(displayOut.String()); got != "Deadcode..." {
		t.Fatalf("expected deadcode start line, got %q", displayOut.String())
	}
	if displayErr.Len() != 0 {
		t.Fatalf("silent display leaked stderr to display: %q", displayErr.String())
	}
}

func TestDeadcodeFailureDiagnostics(t *testing.T) {
	ctx := context.Background()
	failInner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", gatetest.FailWith(errForcedFailure,
			"example.com/mod/pkg.UnusedFunc\n")),
	)
	var displayOut, displayErr bytes.Buffer
	runner := mustNewDisplayRunner(t, failInner, OutputModeAgent, &displayOut, &displayErr)
	fileOps := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot
	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.31.0", true)

	err := Deadcode(
		ctx,
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
	)
	if err == nil {
		t.Fatal("expected deadcode to fail")
	}
	if got := strings.TrimSpace(displayOut.String()); got != "Deadcode..." {
		t.Fatalf("expected deadcode start line on failure, got %q", displayOut.String())
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if got, want := displayErr.String(), de.Error()+"\n"; got != want {
		t.Fatalf("expected exact diagnostic stderr display\\nwant: %q\\ngot:  %q", want, got)
	}
	if de.Name() != cmdDeadcode {
		t.Errorf("expected name deadcode, got %q", de.Name())
	}
}

func TestDeadcodeVerboseModeFailureReturnsRawErrorWithoutDiagnosticError(t *testing.T) {
	ctx := context.Background()
	failInner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", gatetest.FailWith(errForcedFailure,
			"example.com/mod/pkg.UnusedFunc\n")),
	)
	runner := mustNewDisplayRunner(t, failInner, OutputModeVerbose, io.Discard, io.Discard)
	fileOps := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot
	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("deadcode", "golang.org/x/tools/cmd/deadcode@v0.31.0", true)

	err := Deadcode(
		ctx,
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
	)
	if err == nil {
		t.Fatal("expected deadcode to fail")
	}
	var de *DiagnosticError
	if errors.As(err, &de) {
		t.Fatalf("expected raw verbose error, got diagnostic %v", de)
	}
	if !errors.Is(err, ErrDeadcodeFailed) {
		t.Fatalf("expected ErrDeadcodeFailed, got %v", err)
	}
}
