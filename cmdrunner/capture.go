// Vision: Capture wraps any CommandRunner to buffer stdout/stderr/exit for steps that need post-hoc inspection.
package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// CommandResult is filled from buffers after Run; Stdout/Stderr include partial output on failure.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Capture runs the command and returns populated CommandResult plus err from the runner.
// ExitCode is 0 on success; on failure mirrors *exec.ExitError when present, else -1.
// Inspect err before trusting ExitCode.
func Capture(ctx context.Context, runner CommandRunner, dir, name string, args ...string) (CommandResult, error) {
	if runner == nil {
		return CommandResult{ExitCode: -1}, fmt.Errorf("%w: CommandRunner cannot be nil", ErrNilDependency)
	}
	var outBuf, errBuf bytes.Buffer
	err := runner.Run(ctx, dir, &outBuf, &errBuf, name, args...)
	result := CommandResult{Stdout: outBuf.String(), Stderr: errBuf.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, err
	}
	return result, nil
}
