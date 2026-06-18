// Vision: runCommand internals: argv/env/cwd plumbing and error paths without touching the real filesystem.
package harness

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

var errSimulatedFailure = errors.New("simulated failure")

func mustHarness(tb testing.TB, runner cmdrunner.CommandRunner) *StepRunner {
	tb.Helper()
	harn, err := NewStepRunner(
		"/test-root",
		"test-artifacts",
		"",
		runner,
		gatetest.NewMemoryFileOps(),
		NewDiscardArtifactStore(),
		"",
		WithToolResolver(gatetest.NewFakeToolResolver().SetDefaultToLocal(true)),
	)
	if err != nil {
		tb.Fatalf("NewStepRunner: %v", err)
	}
	return harn
}

func TestRunCommand_Success(t *testing.T) {
	t.Parallel()
	var sawName string
	var sawArgs []string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", func(_ context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
			sawName = cmd.Name()
			sawArgs = append([]string(nil), cmd.Args()...)
			_, _ = io.WriteString(stdout, "ok-out\n")
			_, _ = io.WriteString(stderr, "ok-err\n")
			return nil
		}),
	)
	h := mustHarness(t, runner)
	result, err := h.runCommand(context.Background(), "/work", "go", "build", "./...")
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if sawName != "go" {
		t.Fatalf("name = %q, want go", sawName)
	}
	if !slices.Equal(sawArgs, []string{"build", "./..."}) {
		t.Fatalf("args = %v", sawArgs)
	}
	if result.Stdout != "ok-out\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok-out\n")
	}
	if result.Stderr != "ok-err\n" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "ok-err\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRunCommand_FailureCapturesStreams(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", func(_ context.Context, _ cmdrunner.Command, stdout, stderr io.Writer) error {
			_, _ = io.WriteString(stdout, "bad-out")
			_, _ = io.WriteString(stderr, "bad-err")
			return errSimulatedFailure
		}),
	)
	h := mustHarness(t, runner)
	result, err := h.runCommand(context.Background(), "/", "go", "build", ".")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errSimulatedFailure) {
		t.Fatalf("expected errSimulatedFailure in chain, got %v", err)
	}
	if result.Stdout != "bad-out" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "bad-out")
	}
	if result.Stderr != "bad-err" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "bad-err")
	}
	if result.ExitCode != -1 {
		t.Errorf("ExitCode for non-exec error = %d, want -1", result.ExitCode)
	}
}

func TestRunCommand_StdoutOnlyReturned(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go version", func(_ context.Context, _ cmdrunner.Command, stdout, stderr io.Writer) error {
			_, _ = io.WriteString(stdout, "alpha")
			_, _ = io.WriteString(stderr, "warning: beta")
			return nil
		}),
	)
	h := mustHarness(t, runner)
	result, err := h.runCommand(context.Background(), "/", "go", "version")
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if result.Stdout != "alpha" {
		t.Fatalf("Stdout = %q, want alpha", result.Stdout)
	}
	if result.Stderr != "warning: beta" {
		t.Fatalf("Stderr = %q, want %q", result.Stderr, "warning: beta")
	}
}

func TestRunCommand_ExternalBinarySuccess(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("deadcode", func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, "binary output")
			return nil
		}),
	)
	h := mustHarness(t, runner)
	result, err := h.runCommand(context.Background(), "/", "deadcode", "./...")
	if err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if result.Stdout != "binary output" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "binary output")
	}
}

func TestRunCommand_ExternalBinaryFailure(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("golangci-lint", func(_ context.Context, _ cmdrunner.Command, stdout, stderr io.Writer) error {
			_, _ = io.WriteString(stdout, "lint findings")
			_, _ = io.WriteString(stderr, "error detail")
			return errSimulatedFailure
		}),
	)
	h := mustHarness(t, runner)
	result, err := h.runCommand(context.Background(), "/", "golangci-lint", "run")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Stdout != "lint findings" {
		t.Errorf("Stdout on failure = %q, want %q", result.Stdout, "lint findings")
	}
	if result.Stderr != "error detail" {
		t.Errorf("Stderr on failure = %q, want %q", result.Stderr, "error detail")
	}
	if result.ExitCode != -1 {
		t.Errorf("ExitCode for non-exec error = %d, want -1", result.ExitCode)
	}
}
