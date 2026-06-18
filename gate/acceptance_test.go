// Vision: End-to-end gate flows: package/quality scope, build/test flags, output modes, and multi-suite orchestration.
package gate

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestRunnerInjectionPreventsRealExec(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	fakeResolver := gatetest.NewFakeToolResolver()
	fakeResolver.SetLocalMatch("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)

	err := Lint(
		ctx,
		runner,
		fakeResolver,
		fileOps,
		root,
		pkgScope,
		testDefaultLintToolchain(t),
	)
	if err != nil {
		t.Fatalf("Lint() failed: %v", err)
	}
	err = Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Fatal("expected go build to be called")
	}
	foundLint := false
	const golangciLintBinary = "golangci-lint"
	for _, cmd := range inner.Calls() {
		if cmd.Name() == golangciLintBinary {
			foundLint = true
			break
		}
	}
	if !foundLint {
		t.Fatal("expected golangci-lint to be called")
	}
}

func TestExcludeRespectedInCoverpkg(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	scope, err := NewQualityScope("./...", Exclude("features", "testdata", "tools"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope := mustNewPackageScope(t, "./...")
	inv := mustQualityScopeInventoryForTests(t, runner, store, fileOps, root, scope)

	_, err = CoveredTest(ctx, runner, store, fileOps, root, pkgScope, scope, inv)
	if err != nil {
		t.Fatalf("CoveredTest() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdTest) {
		t.Fatal("expected go test to be called")
	}
	for _, cmd := range inner.Calls() {
		if cmd.Name() != cmdGo || cmd.Arg(0) != cmdTest {
			continue
		}
		if !hasArg(cmd, "-json") {
			t.Error("expected -json flag in go test args")
		}
		if !hasArgPrefix(cmd, "-coverprofile=") {
			t.Error("expected -coverprofile flag for CoveredTest")
		}
	}
}

func TestTwoScopesTwoTests(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	scope1, err := NewQualityScope("./gate/...")
	if err != nil {
		t.Fatalf("NewQualityScope(scope1) failed: %v", err)
	}
	scope2, err := NewQualityScope("./internal/...")
	if err != nil {
		t.Fatalf("NewQualityScope(scope2) failed: %v", err)
	}

	pkg1, err := NewPackageScope(scope1.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope(scope1): %v", err)
	}
	pkg2, err := NewPackageScope(scope2.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope(scope2): %v", err)
	}
	out1, err := Test(ctx, runner, store, fileOps, root, pkg1)
	if err != nil {
		t.Fatalf("Test(scope1) failed: %v", err)
	}
	out2, err := Test(ctx, runner, store, fileOps, root, pkg2)
	if err != nil {
		t.Fatalf("Test(scope2) failed: %v", err)
	}

	if out1.stepID == out2.stepID {
		t.Fatal("expected different step IDs for different scopes")
	}

	testCalls := countGoCall(inner.Calls(), cmdTest)
	if testCalls != 2 {
		t.Fatalf("expected 2 go test calls, got %d", testCalls)
	}
}

func TestNewQualityScopeEmptyReturnsError(t *testing.T) {
	_, err := NewQualityScope("")
	if !errors.Is(err, ErrQualityScopeEmpty) {
		t.Fatalf("expected ErrQualityScopeEmpty, got %v", err)
	}
}

func TestQualityScopePackagesReturnsPattern(t *testing.T) {
	t.Parallel()
	ss, err := NewQualityScope("./gate/...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	if got := ss.Packages(); got != "./gate/..." {
		t.Fatalf("expected ./gate/..., got %s", got)
	}
}

func TestNewQualityScopeRejectsLeadingDash(t *testing.T) {
	_, err := NewQualityScope("-toolexec")
	if err == nil {
		t.Fatal("expected error for package pattern starting with '-'")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationSitesUsesScopePackages(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	scope, err := NewQualityScope("./gate/...", Exclude("testdata"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}

	resolver := gatetest.NewFakeToolResolver()
	store := NewArtifactStore()
	inv := mustQualityScopeInventoryForTests(t, runner, store, fileOps, root, scope)
	mr, err := NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	scanOut, err := mr.Scan(ctx, root, scope, inv, testGremlinsTool)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := MutationSites(scanOut, MaxSites(50)); err != nil {
		t.Fatalf("MutationSites() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdList) {
		t.Error("expected go list to be called for mutation coverpkg resolution")
	}
}

func TestCompileWithCompileArgs(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope, CompileArgs("-race", "-trimpath"))
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Fatal("expected go build to be called")
	}
	for _, cmd := range inner.Calls() {
		if cmd.Name() != cmdGo || cmd.Arg(0) != cmdBuild {
			continue
		}
		if !hasArg(cmd, "-race") {
			t.Error("expected -race in build args")
		}
		if !hasArg(cmd, "-trimpath") {
			t.Error("expected -trimpath in build args")
		}
	}
}

func TestCompileDefaultNoModVerify(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go build", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	for _, cmd := range inner.Calls() {
		if cmd.Name() != cmdGo || cmd.Arg(0) != cmdBuild {
			continue
		}
		if hasArg(cmd, "mod") || hasArg(cmd, "verify") {
			t.Error("build should not include 'go mod verify'")
		}
	}
}

func TestTestArgsAppendedAfterGateFlags(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}

	_, err = Test(ctx, runner, store, fileOps, root, pkgScope, TestArgs("-run", "TestSpecific", "-v"))
	if err != nil {
		t.Fatalf("Test() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdTest) {
		t.Fatal("expected go test to be called")
	}
	for _, cmd := range inner.Calls() {
		if cmd.Name() != cmdGo || cmd.Arg(0) != cmdTest {
			continue
		}
		assertTestArgsOrder(t, cmd)
		assertRequiredTestFlags(t, cmd)
	}
}

func TestVetWithVetArgs(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go vet", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Vet(ctx, runner, fileOps, root, pkgScope, VetArgs("-all"))
	if err != nil {
		t.Fatalf("Vet() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdVet) {
		t.Fatal("expected go vet to be called")
	}
	for _, cmd := range inner.Calls() {
		if cmd.Name() != cmdGo || cmd.Arg(0) != cmdVet {
			continue
		}
		if !hasArg(cmd, "-all") {
			t.Error("expected -all in vet args")
		}
	}
}

func TestVetDefaultRunsGoVet(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem, cmdtest.On("go vet", gatetest.NoopCommand))
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Vet(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Vet() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdVet) {
		t.Fatal("expected go vet to be called")
	}
}

func TestVetWithOtherSteps(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem,
		cmdtest.On("go vet", gatetest.NoopCommand),
		cmdtest.On("go build", gatetest.NoopCommand),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := mem
	root := fakeRoot
	ctx := context.Background()

	pkgScope := mustNewPackageScope(t, "./...")
	err := Vet(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Vet() failed: %v", err)
	}
	err = Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdVet) {
		t.Error("expected go vet to be called")
	}
	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Error("expected go build to be called")
	}
}

func TestDeadcodeInjectionPreventsRealExec(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := fakeTestModuleRoot
	inner := acceptanceFakeRunner(mem,
		cmdtest.On("deadcode", gatetest.NoopCommand),
		cmdtest.On("go build", gatetest.NoopCommand),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
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
	err = Compile(ctx, runner, fileOps, root, pkgScope)
	if err != nil {
		t.Fatalf("Compile() failed: %v", err)
	}

	if !hasGoCall(inner.Calls(), cmdBuild) {
		t.Fatal("expected go build to be called")
	}
	foundDeadcode := false
	for _, cmd := range inner.Calls() {
		if cmd.Name() == "deadcode" {
			foundDeadcode = true
			break
		}
	}
	if !foundDeadcode {
		t.Fatal("expected deadcode to be called")
	}
}
