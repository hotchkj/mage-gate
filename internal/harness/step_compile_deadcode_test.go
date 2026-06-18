// Vision: Compile and Deadcode harness steps: command lines, artifacts, and failures under FakeRunner + mem FileOps.
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

func TestStepCompile_Success(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepCompile(context.Background(), nil); err != nil {
		t.Fatalf("StepCompile failed: %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(calls))
	}
	if calls[0].Name() != "go" {
		t.Fatalf("expected go command, got %q", calls[0].Name())
	}
	want := []string{"build", testPackages}
	if !slices.Equal(calls[0].Args(), want) {
		t.Fatalf("go args = %v, want %v", calls[0].Args(), want)
	}
}

func TestStepCompile_DefaultPackages(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, "", deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepCompile(context.Background(), nil); err != nil {
		t.Fatalf("StepCompile failed: %v", err)
	}
	want := []string{"build", testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("go args = %v, want %v", gotArgs, want)
	}
}

func TestStepCompile_Failure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return errSimulatedFailure
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepCompile(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for failing build")
	}
	if !errors.Is(err, h.ErrCompileFailed) {
		t.Fatalf("expected ErrCompileFailed, got %v", err)
	}
}

func TestStepDeadcode_Clean(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, "")
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepDeadcode(context.Background(), "", nil); err != nil {
		t.Fatalf("StepDeadcode failed: %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(calls))
	}
	if calls[0].Name() != "deadcode" {
		t.Fatalf("expected deadcode command, got %q", calls[0].Name())
	}
	want := []string{testPackages}
	if !slices.Equal(calls[0].Args(), want) {
		t.Fatalf("deadcode args = %v, want %v", calls[0].Args(), want)
	}
}

func TestStepDeadcode_Unreachable(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, "pkg/foo.go:10: unreachable func: Foo\n")
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDeadcode(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDeadcodeFailed) {
		t.Fatalf("expected ErrDeadcodeFailed, got %v", err)
	}
}

func TestStepDeadcode_WithTags(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
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
	if err := harness.StepDeadcode(context.Background(), "", []string{"-tags=integration"}); err != nil {
		t.Fatalf("StepDeadcode failed: %v", err)
	}
	want := []string{"-tags=integration", testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("deadcode args = %v, want %v", gotArgs, want)
	}
}

func TestStepDeadcode_CommandFailure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return errSimulatedFailure
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	err = harness.StepDeadcode(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDeadcodeFailed) {
		t.Fatalf("expected ErrDeadcodeFailed, got %v", err)
	}
}

func TestStepDeadcode_DefaultPackages(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			_, _ = io.WriteString(stdout, "")
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, "", deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepDeadcode(context.Background(), "", nil); err != nil {
		t.Fatalf("StepDeadcode failed: %v", err)
	}
	want := []string{testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("deadcode args = %v, want %v", gotArgs, want)
	}
}
