// Vision: Response factories: stdout/stderr templates, exit codes, and edge cases without a full runner graph.
package cmdtest_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
)

var (
	errTestFailure    = errors.New("test failure")
	errTestLintFailed = errors.New("lint failed")
	errTestWriter     = errors.New("writer error")
)

// errorWriter is an io.Writer that always returns an error
type errorWriter struct {
	err error
}

func (ew errorWriter) Write(p []byte) (int, error) {
	return 0, ew.err
}

func TestNoopCommand_Succeeds(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "vet")
	var out, errOut strings.Builder
	err := cmdtest.NoopCommand(context.Background(), cmd, &out, &errOut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestFail_ReturnsError(t *testing.T) {
	t.Parallel()
	fn := cmdtest.Fail(errTestFailure)
	cmd := cmdrunner.NewCommand(".", "go", "test")
	var out, errOut strings.Builder
	err := fn(context.Background(), cmd, &out, &errOut)
	if !errors.Is(err, errTestFailure) {
		t.Fatalf("error = %v, want %v", err, errTestFailure)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestFailWith_WritesOutputAndReturnsError(t *testing.T) {
	t.Parallel()
	fn := cmdtest.FailWith(errTestLintFailed, "main.go:1:1: error: missing return\n")
	cmd := cmdrunner.NewCommand(".", "golangci-lint", "run")
	var out, errOut strings.Builder
	err := fn(context.Background(), cmd, &out, &errOut)
	if !errors.Is(err, errTestLintFailed) {
		t.Fatalf("error = %v, want %v", err, errTestLintFailed)
	}
	if out.String() != "main.go:1:1: error: missing return\n" {
		t.Fatalf("stdout = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestFailWith_WriterError(t *testing.T) {
	t.Parallel()
	fn := cmdtest.FailWith(errTestLintFailed, "some output")
	cmd := cmdrunner.NewCommand(".", "golangci-lint", "run")
	err := fn(context.Background(), cmd, errorWriter{errTestWriter}, io.Discard)
	// FailWith returns writer errors immediately instead of swallowing them
	if !errors.Is(err, errTestWriter) {
		t.Fatalf("error = %v, want %v", err, errTestWriter)
	}
}
