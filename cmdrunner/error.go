// Vision: DiagnosticError carries stable ERROR/Fix/Hint fields for agent consumers while remaining errors.Is-friendly.
package cmdrunner

import (
	"errors"
	"fmt"
	"strings"
)

// DiagnosticError provides structured error context for tool failures.
// The canonical output format is:
//
//	ERROR: [name] <description>
//	Fix: <remediation>
//	Hint: <guidance>
//	<tool output>
type DiagnosticError struct {
	name       string
	message    string
	fix        string
	hint       string
	toolOutput string
	cause      error
}

// Error formats the diagnostic in ERROR/Fix/Hint format for human diagnostic output.
// This format is optimized for readability; it is not a stable machine-parseable protocol.
// Use errors.Is / errors.As with Unwrap() for programmatic error classification.
func (e *DiagnosticError) Error() string {
	var sb strings.Builder
	if e.name != "" {
		fmt.Fprintf(&sb, "ERROR: [%s] %s\n", e.name, e.message)
	} else {
		fmt.Fprintf(&sb, "ERROR: %s\n", e.message)
	}
	if e.fix != "" {
		fmt.Fprintf(&sb, "Fix: %s\n", e.fix)
	}
	if e.hint != "" {
		fmt.Fprintf(&sb, "Hint: %s\n", e.hint)
	}
	if e.toolOutput != "" {
		sb.WriteString(e.toolOutput)
	}
	return sb.String()
}

func (e *DiagnosticError) Name() string       { return e.name }
func (e *DiagnosticError) Message() string    { return e.message }
func (e *DiagnosticError) Fix() string        { return e.fix }
func (e *DiagnosticError) Hint() string       { return e.hint }
func (e *DiagnosticError) ToolOutput() string { return e.toolOutput }
func (e *DiagnosticError) Unwrap() error      { return e.cause }

// DiagnosticOptions configures optional fields for NewDiagnosticError.
type DiagnosticOptions struct {
	ToolOutput string
	Cause      error
}

// NewDiagnosticError creates a diagnostic error with the ERROR/Fix/Hint format.
// Pass opts nil when no tool output or wrapped cause is needed.
func NewDiagnosticError(name, message, fix, hint string, opts *DiagnosticOptions) error {
	de := &DiagnosticError{
		name:    name,
		message: message,
		fix:     fix,
		hint:    hint,
	}
	if opts != nil {
		de.toolOutput = opts.ToolOutput
		de.cause = opts.Cause
	}
	return de
}

// WrapDiagnostic wraps a non-DiagnosticError into a structured DiagnosticError.
// If err is already a *DiagnosticError, it is returned as-is.
func WrapDiagnostic(name string, err error) error {
	if err == nil {
		return nil
	}
	var de *DiagnosticError
	if errors.As(err, &de) {
		return err
	}
	return NewDiagnosticError(
		name,
		fmt.Sprintf("%s failed", name),
		fmt.Sprintf("review %s configuration", name),
		fmt.Sprintf("see %s output for details", name),
		&DiagnosticOptions{ToolOutput: err.Error(), Cause: err},
	)
}
