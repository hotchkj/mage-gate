// Vision: Capture helper: stdout/stderr plumbing, exit codes, and context cancellation vs real commands.
package cmdrunner_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

var errCaptureTestFailed = errors.New("capture test command failed")

type captureTestRunner struct {
	stdout     string
	stderr     string
	shouldFail bool
}

func (r *captureTestRunner) Run(
	ctx context.Context,
	_ string,
	stdout, stderr io.Writer,
	_ string,
	_ ...string,
) error {
	select {
	case <-ctx.Done():
		_, _ = io.WriteString(stdout, r.stdout)
		return ctx.Err()
	default:
	}
	if _, err := io.WriteString(stdout, r.stdout); err != nil {
		return err
	}
	if _, err := io.WriteString(stderr, r.stderr); err != nil {
		return err
	}
	if r.shouldFail {
		return errCaptureTestFailed
	}
	return nil
}

func TestCapture_Success(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{stdout: "hello", stderr: "warn"}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd", "arg1")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Stdout != "hello" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello")
	}
	if result.Stderr != "warn" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "warn")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestCapture_SuccessEmptyOutput(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Stdout != "" {
		t.Errorf("Stdout = %q, want empty", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", result.Stderr)
	}
}

func TestCapture_FailureReturnsOutputAndError(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{
		stdout:     "partial output",
		stderr:     "error detail",
		shouldFail: true,
	}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errCaptureTestFailed) {
		t.Errorf("error chain should contain errCaptureTestFailed, got: %v", err)
	}
	if result.Stdout != "partial output" {
		t.Errorf("Stdout on failure = %q, want %q", result.Stdout, "partial output")
	}
	if result.Stderr != "error detail" {
		t.Errorf("Stderr on failure = %q, want %q", result.Stderr, "error detail")
	}
	if result.ExitCode != -1 {
		t.Errorf("ExitCode for non-exec error = %d, want -1", result.ExitCode)
	}
}

func TestCapture_FailureWithEmptyOutput(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{shouldFail: true}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errCaptureTestFailed) {
		t.Errorf("expected errCaptureTestFailed in chain, got %v", err)
	}
	if result.Stdout != "" {
		t.Errorf("Stdout = %q, want empty", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", result.Stderr)
	}
}

func TestCapture_NilRunnerReturnsErrNilDependency(t *testing.T) {
	t.Parallel()
	result, err := cmdrunner.Capture(context.Background(), nil, "/dir", "cmd")
	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !errors.Is(err, cmdrunner.ErrNilDependency) {
		t.Errorf("expected ErrNilDependency, got %v", err)
	}
	if result.ExitCode != -1 {
		t.Errorf("ExitCode for nil runner = %d, want -1", result.ExitCode)
	}
}

func TestCapture_StderrOnlySuccess(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{stderr: "warnings only"}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Stdout != "" {
		t.Errorf("Stdout = %q, want empty", result.Stdout)
	}
	if result.Stderr != "warnings only" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "warnings only")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestCapture_ContextCancelledReturnsPartialOutput(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &captureTestRunner{stdout: "before cancel", shouldFail: true}
	result, err := cmdrunner.Capture(ctx, runner, "/dir", "cmd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in error chain, got %v", err)
	}
	if result.Stdout != "before cancel" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "before cancel")
	}
	if result.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", result.ExitCode)
	}
}

func TestCapture_SeparatesStdoutAndStderr(t *testing.T) {
	t.Parallel()
	runner := &captureTestRunner{
		stdout: "stdout content",
		stderr: "stderr content",
	}
	result, err := cmdrunner.Capture(context.Background(), runner, "/dir", "cmd")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Stdout != "stdout content" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "stdout content")
	}
	if result.Stderr != "stderr content" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "stderr content")
	}
}
