// Vision: Vet step: consumer-supplied vet flags, package scope argv, and tool output handling under fakes.
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

func TestStepVet_Success(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", gatetest.NoopCommand),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepVet(context.Background(), nil); err != nil {
		t.Fatalf("StepVet failed: %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(calls))
	}
	if calls[0].Name() != "go" {
		t.Fatalf("expected go command, got %q", calls[0].Name())
	}
	want := []string{"vet", testPackages}
	if !slices.Equal(calls[0].Args(), want) {
		t.Fatalf("go args = %v, want %v", calls[0].Args(), want)
	}
}

func TestStepVet_Failure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return errSimulatedFailure
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepVet(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrVetFailed) {
		t.Fatalf("expected ErrVetFailed, got %v", err)
	}
}

func TestStepVet_EmptyPackagesUsesDefaultScope(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, "", deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepVet(context.Background(), nil); err != nil {
		t.Fatalf("StepVet failed: %v", err)
	}
	want := []string{"vet", testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("go args = %v, want %v", gotArgs, want)
	}
}

func TestStepVet_NonEmptyPackages(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, "./mypkg/...", deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepVet(context.Background(), nil); err != nil {
		t.Fatalf("StepVet failed: %v", err)
	}
	want := []string{"vet", "./mypkg/..."}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("go args = %v, want %v", gotArgs, want)
	}
}

func TestStepVet_WithVetArgs(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepVet(context.Background(), []string{"-printfuncs", "Infof"}); err != nil {
		t.Fatalf("StepVet failed: %v", err)
	}
	want := []string{"vet", "-printfuncs", "Infof", testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("go args = %v, want %v", gotArgs, want)
	}
}

func TestStepVet_WithoutVetArgs(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go vet", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			gotArgs = append([]string(nil), cmd.Args()...)
			return nil
		}),
	)
	deps := validDeps(runner)
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepVet(context.Background(), nil); err != nil {
		t.Fatalf("StepVet failed: %v", err)
	}
	want := []string{"vet", testPackages}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("go args = %v, want %v", gotArgs, want)
	}
}
