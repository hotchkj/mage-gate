// Vision: Shared fixture layout, runner construction, and assertions for acceptance_*_test.go scenarios.
package gate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

// Shared fake module metadata for gatetest.GoList and GoTestPass in acceptance tests.
const (
	fakeModulePath        = "github.com/hotchkj/mage-gate"
	fakeGateGoTestPackage = "github.com/hotchkj/mage-gate/gate"
	fakeImportPathHarness = "github.com/hotchkj/mage-gate/internal/harness"
)

// fakeGremlinsReport matches gremlins unleash JSON output (mutation site budget).
// Gremlins fakes use the same synthetic root as the harness so relative -o
// paths land on the same canonical logical artifact paths read through FileOps.
var fakeGremlinsReport = []byte(`{"files":[{"file_name":"pkg/foo.go","mutations":[]}]}`)

// Sentinel errors for test assertions (err113 compliance).
var errForcedFailure = fmt.Errorf("exit status 1")

func assertErrorIs(tb testing.TB, err, want error) {
	tb.Helper()
	if err == nil {
		tb.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		tb.Fatalf("expected errors.Is(err, %v), got %v", want, err)
	}
}

// Command name constants for test assertions (goconst compliance).
const (
	cmdGo       = "go"
	cmdTest     = "test"
	cmdBuild    = "build"
	cmdVet      = "vet"
	cmdList     = "list"
	cmdDeadcode = "deadcode"
)

// fakeTestModuleRoot is the synthetic module root for tests that use in-memory file ops.
const fakeTestModuleRoot = "/fake-root"

// acceptanceFakeRunnerForRoot is like acceptanceFakeRunner but uses the given synthetic module root
// for go list directory mapping (tests that use a non-default fake root).
func acceptanceFakeRunnerForRoot(
	mem gatetest.FileOpsWriter,
	root string,
	opts ...cmdtest.RunnerOption,
) *cmdtest.FakeRunner {
	return cmdtest.NewFakeRunner(append([]cmdtest.RunnerOption{
		cmdtest.On("go test", gatetest.GoTestPassWithCoverage(mem, fakeGateGoTestPackage, 100)),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(
			map[string]float64{"github.com/hotchkj/mage-gate/file.go:10:\tValidate": 100.0},
			100.0,
		)),
		cmdtest.On("go list", gatetest.GoList(fakeModulePath, root, map[string]gatetest.PackageListInfo{
			fakeGateGoTestPackage: gatetest.DirOnly(filepath.Join(root, "gate")),
			fakeImportPathHarness: gatetest.DirOnly(filepath.Join(root, "internal", "harness")),
		})),
		cmdtest.On("go run", gatetest.Gremlins(mem, root, fakeGremlinsReport)),
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
	}, opts...)...)
}

// acceptanceFakeRunner returns the shared FakeRunner wiring used across acceptance tests
// (go test, cover, list, gremlins, golangci-lint). Pass extra On() options for step-specific tools
// such as go build or go vet.
func acceptanceFakeRunner(mem gatetest.FileOpsWriter, opts ...cmdtest.RunnerOption) *cmdtest.FakeRunner {
	return acceptanceFakeRunnerForRoot(mem, fakeTestModuleRoot, opts...)
}

func hasGoCall(calls []cmdrunner.Command, subcmd string) bool {
	for _, cmd := range calls {
		if cmd.Name() == cmdGo && cmd.Arg(0) == subcmd {
			return true
		}
	}
	return false
}

func hasArg(cmd cmdrunner.Command, arg string) bool {
	for _, a := range cmd.Args() {
		if a == arg {
			return true
		}
	}
	return false
}

func hasArgPrefix(cmd cmdrunner.Command, prefix string) bool {
	for _, a := range cmd.Args() {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func countGoCall(calls []cmdrunner.Command, subcmd string) int {
	count := 0
	for _, cmd := range calls {
		if cmd.Name() == cmdGo && cmd.Arg(0) == subcmd {
			count++
		}
	}
	return count
}

func argIndex(cmd cmdrunner.Command, arg string) int {
	for idx, a := range cmd.Args() {
		if a == arg {
			return idx
		}
	}
	return -1
}

func assertTestArgsOrder(t *testing.T, cmd cmdrunner.Command) {
	t.Helper()
	jsonIdx := argIndex(cmd, "-json")
	count1Idx := argIndex(cmd, "-count=1")
	runIdx := argIndex(cmd, "-run")
	if jsonIdx == -1 {
		t.Fatalf("expected -json flag")
	}
	if count1Idx == -1 {
		t.Fatalf("expected -count=1 flag")
	}
	if runIdx == -1 {
		t.Fatalf("expected -run flag from TestArgs")
	}
	if jsonIdx != -1 && count1Idx != -1 && count1Idx < jsonIdx {
		t.Fatalf("expected -json before -count=1")
	}
	// -count=1 is the last flag; consumer flags (e.g. -run) must precede it
	if count1Idx != -1 && runIdx != -1 && count1Idx < runIdx {
		t.Fatalf("expected consumer -run flag before -count=1 (which must be last)")
	}
}

// assertRequiredTestFlags verifies the harness unconditionally injects -count=1 and does
// not inject -shuffle (a consumer preference passed via TestArgs, not a correctness invariant).
func assertRequiredTestFlags(t *testing.T, cmd cmdrunner.Command) {
	t.Helper()
	if !hasArg(cmd, "-count=1") {
		t.Fatalf("expected -count=1 to be injected by harness")
	}
	if hasArg(cmd, "-shuffle=on") {
		t.Fatalf("expected -shuffle=on to be absent without TestArgs option")
	}
}

func mustNewQualityScope(t *testing.T, pkgs string) QualityScope {
	t.Helper()
	scope, err := NewQualityScope(pkgs)
	if err != nil {
		t.Fatalf("NewQualityScope(%q) failed: %v", pkgs, err)
	}
	return scope
}

func mustNewPackageScope(t *testing.T, pkgs string) PackageScope {
	t.Helper()
	p, err := NewPackageScope(pkgs)
	if err != nil {
		t.Fatalf("NewPackageScope(%q) failed: %v", pkgs, err)
	}
	return p
}

func mustCoveredTestOutput(t *testing.T, out *CoveredTestOutput) CoveredTestOutput {
	t.Helper()
	if out == nil {
		t.Fatalf("mustCoveredTestOutput: result is nil")
	}
	if out.stepID == "" {
		t.Fatalf("mustCoveredTestOutput: stepID is empty")
	}
	if out.packages.Packages() == "" {
		t.Fatalf("mustCoveredTestOutput: packages is empty")
	}
	if qualityScopePackages(out.qualityScope) == "" {
		t.Fatalf("mustCoveredTestOutput: qualityScope.packages is empty")
	}
	return *out
}

func mustTestOutputFromCovered(tb testing.TB, out *CoveredTestOutput) TestOutput {
	tb.Helper()
	if out == nil {
		tb.Fatal("CoveredTestOutput is nil")
	}
	tr, err := out.TestRun()
	if err != nil {
		tb.Fatalf("CoveredTestOutput.TestRun: %v", err)
	}
	return tr
}
