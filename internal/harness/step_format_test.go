// Vision: Format step: golangci-lint fmt command construction under fakes.
package harness_test

import (
	"context"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

const testGolangCILintBinary = "golangci-lint"

func TestStepFormat_Success(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On(testGolangCILintBinary, gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepFormat(context.Background(), testLintConfig, "", "", testLintToolSpec, nil); err != nil {
		t.Fatalf("StepFormat: %v", err)
	}
	calls := runner.Calls()
	var formatCall cmdrunner.Command
	for _, c := range calls {
		if c.Name() == testGolangCILintBinary && slices.Contains(c.Args(), "fmt") {
			formatCall = c
			break
		}
	}
	if formatCall.Name() == "" {
		t.Fatalf("expected golangci-lint fmt call; got %v", calls)
	}
	assertFlagFollowedBy(t, formatCall.Args(), "-c", testLintConfig)
}
