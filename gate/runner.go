// Vision: Runner factory and output mode for gate consumers; see docs/mage-gate-intent-and-design.md §8.
package gate

import (
	"context"
	"fmt"
	"io"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

type CommandRunner = cmdrunner.CommandRunner

type ToolResolver = cmdrunner.ToolResolver

type OutputMode string

const (
	OutputModeAgent   OutputMode = "agent"
	OutputModeVerbose OutputMode = "verbose"
)

type silentWriter struct{}

func (s silentWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func outputWriter(mode OutputMode, defaultWriter io.Writer) io.Writer {
	if mode == OutputModeAgent {
		return silentWriter{}
	}
	return defaultWriter
}

type displayRunner struct {
	inner      CommandRunner
	mode       OutputMode
	displayOut io.Writer
	displayErr io.Writer
}

type stepDisplay interface {
	EmitStepStartLine(line string)
	EmitDiagnostic(diagnostic string)
}

func runnerAsStepDisplay(runner CommandRunner) stepDisplay {
	display, ok := runner.(stepDisplay)
	if !ok {
		return nil
	}
	return display
}

// NewDisplayRunner tees capture writers to inner Run with per-mode display filtering (see outputWriter).
func NewDisplayRunner(inner CommandRunner, mode OutputMode, displayOut, displayErr io.Writer) (CommandRunner, error) {
	if inner == nil {
		return nil, fmt.Errorf("%w: CommandRunner cannot be nil", ErrNilDependency)
	}
	if displayOut == nil {
		return nil, fmt.Errorf("%w: displayOut cannot be nil", ErrNilDependency)
	}
	if displayErr == nil {
		return nil, fmt.Errorf("%w: displayErr cannot be nil", ErrNilDependency)
	}
	switch mode {
	case OutputModeAgent, OutputModeVerbose:
	default:
		return nil, fmt.Errorf(
			"%w: %q (must be %q or %q)",
			ErrInvalidOutputMode,
			mode,
			OutputModeAgent,
			OutputModeVerbose,
		)
	}
	return &displayRunner{inner: inner, mode: mode, displayOut: displayOut, displayErr: displayErr}, nil
}

func (r *displayRunner) Run(
	ctx context.Context,
	dir string,
	stdout, stderr io.Writer,
	name string,
	args ...string,
) error {
	out := r.mergeWriter(stdout, r.displayOut)
	errw := r.mergeWriter(stderr, r.displayErr)
	return r.inner.Run(ctx, dir, out, errw, name, args...)
}

func (r *displayRunner) mergeWriter(capture, display io.Writer) io.Writer {
	filtered := outputWriter(r.mode, display)
	if capture == nil || capture == io.Discard {
		return filtered
	}
	return io.MultiWriter(filtered, capture)
}

func (r *displayRunner) EmitStepStartLine(line string) {
	if r.displayOut == nil || line == "" {
		return
	}
	_, _ = fmt.Fprintln(r.displayOut, line)
}

func (r *displayRunner) EmitDiagnostic(diagnostic string) {
	if r.displayErr == nil || diagnostic == "" {
		return
	}
	_, _ = fmt.Fprintln(r.displayErr, diagnostic)
}

// OutputModeProvider lets custom CommandRunner implementations report display mode for [RunnerOutputMode].
type OutputModeProvider interface {
	RunOutputMode() OutputMode
}

func (r *displayRunner) RunOutputMode() OutputMode {
	return r.mode
}

// RunnerOutputMode is used by step error shaping; nil runners, absent [OutputModeProvider], or
// unrecognized [OutputModeProvider.RunOutputMode] values read as [OutputModeVerbose].
func RunnerOutputMode(r CommandRunner) OutputMode {
	if r == nil {
		return OutputModeVerbose
	}
	p, ok := r.(OutputModeProvider)
	if !ok {
		return OutputModeVerbose
	}
	switch m := p.RunOutputMode(); m {
	case OutputModeAgent, OutputModeVerbose:
		return m
	default:
		return OutputModeVerbose
	}
}

func NewProductionRunner() CommandRunner { return cmdrunner.NewProductionRunner() }

func NewProductionToolResolver() ToolResolver { return cmdrunner.NewProductionToolResolver() }
