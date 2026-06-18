// Vision: Display runner and RunnerOutputMode / wrapStepError behavior.
package gate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

const (
	displayTestOut = "out"
	displayTestErr = "err"
)

var errSimulatedToolFailure = errors.New("simulated tool failure")

type echoDisplayInner struct{}

func (echoDisplayInner) Run(_ context.Context, _ string, stdout, stderr io.Writer, _ string, _ ...string) error {
	if _, err := io.WriteString(stdout, displayTestOut); err != nil {
		return err
	}
	_, err := io.WriteString(stderr, displayTestErr)
	return err
}

func TestRunnerOutputMode_NilRunner(t *testing.T) {
	t.Parallel()
	if got := RunnerOutputMode(nil); got != OutputModeVerbose {
		t.Fatalf("nil runner: got %q, want verbose", got)
	}
}

func TestRunnerOutputMode_BareRunner(t *testing.T) {
	t.Parallel()
	inner := echoDisplayInner{}
	if got := RunnerOutputMode(inner); got != OutputModeVerbose {
		t.Fatalf("bare runner: got %q, want verbose", got)
	}
}

func TestRunnerOutputMode_NewDisplayRunnerEachMode(t *testing.T) {
	t.Parallel()
	inner := echoDisplayInner{}
	silent := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	if got := RunnerOutputMode(silent); got != OutputModeAgent {
		t.Fatalf("silent display: got %q, want silent", got)
	}
	verbose := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)
	if got := RunnerOutputMode(verbose); got != OutputModeVerbose {
		t.Fatalf("verbose display: got %q, want verbose", got)
	}
}

type bogusModeRunner struct{}

func (bogusModeRunner) Run(context.Context, string, io.Writer, io.Writer, string, ...string) error {
	return nil
}

func (bogusModeRunner) RunOutputMode() OutputMode {
	return OutputMode("bogus")
}

func TestRunnerOutputMode_UnrecognizedImplementerReturnsVerbose(t *testing.T) {
	t.Parallel()
	if got := RunnerOutputMode(bogusModeRunner{}); got != OutputModeVerbose {
		t.Fatalf("bogus mode: got %q, want verbose", got)
	}
}

type verboseModeRunner struct{}

func (verboseModeRunner) Run(context.Context, string, io.Writer, io.Writer, string, ...string) error {
	return nil
}

func (verboseModeRunner) RunOutputMode() OutputMode {
	return OutputModeVerbose
}

func TestWrapStepError_VerboseRunnerRawError(t *testing.T) {
	t.Parallel()
	out := wrapStepError("lint", verboseModeRunner{}, errSimulatedToolFailure)
	if !errors.Is(out, errSimulatedToolFailure) {
		t.Fatalf("verbose: want raw error chain, got %v", out)
	}
	var de *DiagnosticError
	if errors.As(out, &de) {
		t.Fatalf("verbose: must not wrap with DiagnosticError, got %v", out)
	}
}

func TestWrapStepError_SilentDisplayRunnerDiagnostic(t *testing.T) {
	t.Parallel()
	inner := echoDisplayInner{}
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	out := wrapStepError("lint", runner, errSimulatedToolFailure)
	var de *DiagnosticError
	if !errors.As(out, &de) {
		t.Fatalf("silent display: want DiagnosticError, got %T: %v", out, out)
	}
}

func TestWrapStepError_NoInterfaceRunnerRawError(t *testing.T) {
	t.Parallel()
	out := wrapStepError("lint", echoDisplayInner{}, errSimulatedToolFailure)
	if !errors.Is(out, errSimulatedToolFailure) {
		t.Fatalf("bare runner: want raw error chain, got %v", out)
	}
	var de *DiagnosticError
	if errors.As(out, &de) {
		t.Fatalf("bare runner: must not wrap with DiagnosticError, got %v", out)
	}
}

func TestNewDisplayRunner_ErrorsAreGateOwned(t *testing.T) {
	t.Parallel()
	inner := echoDisplayInner{}
	_, err := NewDisplayRunner(nil, OutputModeAgent, io.Discard, io.Discard)
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("nil inner: got %v", err)
	}
	if errors.Is(err, cmdrunner.ErrNilDependency) {
		t.Fatal("must not surface cmdrunner.ErrNilDependency")
	}
	_, err = NewDisplayRunner(inner, OutputModeAgent, nil, io.Discard)
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("nil displayOut: got %v", err)
	}
	_, err = NewDisplayRunner(inner, OutputModeAgent, io.Discard, nil)
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("nil displayErr: got %v", err)
	}
	_, err = NewDisplayRunner(inner, OutputMode("nope"), io.Discard, io.Discard)
	if !errors.Is(err, ErrInvalidOutputMode) {
		t.Fatalf("invalid mode: got %v", err)
	}
}

func TestNewDisplayRunner_SilentMode_SilencesDisplayCaptureGetsOutput(t *testing.T) {
	t.Parallel()
	var displayOut, displayErr bytes.Buffer
	var captureOut, captureErr bytes.Buffer
	inner := echoDisplayInner{}
	r := mustNewDisplayRunner(t, inner, OutputModeAgent, &displayOut, &displayErr)
	if err := r.Run(context.Background(), "/", &captureOut, &captureErr, "x"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if displayOut.String() != "" || displayErr.String() != "" {
		t.Fatalf("expected silent display, got stdout=%q stderr=%q", displayOut.String(), displayErr.String())
	}
	if captureOut.String() != displayTestOut || captureErr.String() != displayTestErr {
		t.Fatalf("capture got stdout=%q stderr=%q", captureOut.String(), captureErr.String())
	}
}

func TestNewDisplayRunner_VerboseMode_DisplayAndCaptureReceiveOutput(t *testing.T) {
	t.Parallel()
	var displayOut, displayErr bytes.Buffer
	var captureOut, captureErr bytes.Buffer
	inner := echoDisplayInner{}
	r := mustNewDisplayRunner(t, inner, OutputModeVerbose, &displayOut, &displayErr)
	if err := r.Run(context.Background(), "/", &captureOut, &captureErr, "x"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if displayOut.String() != displayTestOut || displayErr.String() != displayTestErr {
		t.Fatalf("display got stdout=%q stderr=%q", displayOut.String(), displayErr.String())
	}
	if captureOut.String() != displayTestOut || captureErr.String() != displayTestErr {
		t.Fatalf("capture got stdout=%q stderr=%q", captureOut.String(), captureErr.String())
	}
}

func TestOutputWriter_SilentDiscards(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	wr := outputWriter(OutputModeAgent, buf)
	if _, err := wr.Write([]byte("noise")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected silent to discard, got %q", buf.String())
	}
}

func TestOutputWriter_VerboseUsesDefault(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	wr := outputWriter(OutputModeVerbose, buf)
	if _, err := wr.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.String() != "x" {
		t.Fatalf("expected default writer, got %q", buf.String())
	}
}

func TestOutputWriter_UnrecognizedModeForwardsToDefault(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	wr := outputWriter(OutputMode("unknown"), buf)
	if _, err := wr.Write([]byte("data")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.String() != "data" {
		t.Fatalf("expected unrecognized mode to pass through, got %q", buf.String())
	}
}

func TestSilentWriter_WriteReturnsLen(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	wr := outputWriter(OutputModeAgent, buf)
	payload := []byte("hello")
	n, err := wr.Write(payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected n=%d, got %d", len(payload), n)
	}
}
