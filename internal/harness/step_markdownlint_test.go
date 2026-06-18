// Vision: Markdownlint harness step: gomarklint resolution, consumer argv passthrough, and failures under fakes.
package harness_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

const testMarkdownlintToolSpec = "github.com/shinagawa-web/gomarklint/v3@v3.2.3"

func TestStepMarkdownLint_Clean(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("gomarklint", func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, "")
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, nil); err != nil {
		t.Fatalf("StepMarkdownLint failed: %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(calls))
	}
	if calls[0].Name() != "gomarklint" {
		t.Fatalf("expected gomarklint command, got %q", calls[0].Name())
	}
	if len(calls[0].Args()) != 0 {
		t.Fatalf("gomarklint args = %v, want empty", calls[0].Args())
	}
}

func TestStepMarkdownLint_CommandFailure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("gomarklint", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return errSimulatedFailure
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrMarkdownLintFailed) {
		t.Fatalf("expected ErrMarkdownLintFailed, got %v", err)
	}
}

func TestStepMarkdownLint_ArgsPassthrough(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("gomarklint", func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			_, _ = io.WriteString(stdout, "")
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	consumerArgs := []string{"--config", ".gomarklint.json"}
	if err := harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, consumerArgs); err != nil {
		t.Fatalf("StepMarkdownLint failed: %v", err)
	}
	want := []string{"--config", ".gomarklint.json"}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("gomarklint args = %v, want %v", gotArgs, want)
	}
}

func TestStepMarkdownLint_NilToolResolver(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner()
	harness, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir, testPackages,
		runner, validFileOps(), h.NewDiscardArtifactStore(), "",
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error when ToolResolver is nil")
	}
	if !errors.Is(err, h.ErrMarkdownLintFailed) {
		t.Fatalf("expected ErrMarkdownLintFailed, got %v", err)
	}
}

func TestStepMarkdownLint_ResolveFailure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner()
	resolver := gatetest.NewFakeToolResolverError("injected failure")
	harness, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir, testPackages,
		runner, validFileOps(), h.NewDiscardArtifactStore(), "",
		h.WithToolResolver(resolver),
	)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	err = harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, nil)
	if err == nil {
		t.Fatal("expected error for resolve failure")
	}
	if !errors.Is(err, h.ErrMarkdownLintFailed) {
		t.Fatalf("expected ErrMarkdownLintFailed, got %v", err)
	}
}

func TestStepMarkdownLint_UsesGomarklintProbe(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("gomarklint", gatetest.NoopCommand),
	)
	resolver := gatetest.NewFakeToolResolver().SetDefaultToLocal(true)
	deps := testHarnessDeps{
		Runner:  runner,
		FileOps: validFileOps(),
		Options: []h.StepRunnerOption{h.WithToolResolver(resolver)},
	}
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepMarkdownLint(context.Background(), testMarkdownlintToolSpec, nil); err != nil {
		t.Fatalf("StepMarkdownLint failed: %v", err)
	}
	resolverCalls := resolver.Calls()
	if len(resolverCalls) != 1 {
		t.Fatalf("expected 1 resolver call, got %d", len(resolverCalls))
	}
	if resolverCalls[0].ToolName != "gomarklint" {
		t.Fatalf("resolver tool name = %q, want %q", resolverCalls[0].ToolName, "gomarklint")
	}
	if resolverCalls[0].ToolSpec != testMarkdownlintToolSpec {
		t.Fatalf("resolver tool spec = %q, want %q", resolverCalls[0].ToolSpec, testMarkdownlintToolSpec)
	}
}
