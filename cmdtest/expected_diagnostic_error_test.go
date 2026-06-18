package cmdtest_test

import (
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
)

type expectedDiagnosticErrorCase struct {
	name       string
	diagName   string
	message    string
	fix        string
	hint       string
	toolOutput string
}

func expectedDiagnosticErrorCases() []expectedDiagnosticErrorCase {
	const (
		nameValue   = "compile"
		message     = "compile failed"
		fixValue    = "fix compile errors"
		hintValue   = "check compiler output"
		toolOutput  = "pkg/a.go:1:1: undefined: x"
		emptyName   = ""
		emptyFix    = ""
		emptyHint   = ""
		emptyOutput = ""
	)

	return []expectedDiagnosticErrorCase{
		{
			name:       "full_fields",
			diagName:   nameValue,
			message:    message,
			fix:        fixValue,
			hint:       hintValue,
			toolOutput: toolOutput,
		},
		{
			name:     "empty_name",
			diagName: emptyName,
			message:  message,
			fix:      fixValue,
			hint:     hintValue,
		},
		{
			name:       "tool_output_only",
			diagName:   nameValue,
			message:    message,
			fix:        emptyFix,
			hint:       emptyHint,
			toolOutput: toolOutput,
		},
		{
			name:       "empty_message",
			diagName:   nameValue,
			message:    "",
			fix:        fixValue,
			hint:       hintValue,
			toolOutput: toolOutput,
		},
		{
			name:       "all_fields_empty",
			diagName:   emptyName,
			message:    "",
			fix:        emptyFix,
			hint:       emptyHint,
			toolOutput: emptyOutput,
		},
	}
}

func TestExpectedDiagnosticErrorStringMatchesDiagnosticError(t *testing.T) {
	t.Parallel()

	for _, tc := range expectedDiagnosticErrorCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cmdrunner.NewDiagnosticError(
				tc.diagName,
				tc.message,
				tc.fix,
				tc.hint,
				&cmdrunner.DiagnosticOptions{ToolOutput: tc.toolOutput},
			)
			want := cmdtest.ExpectedDiagnosticErrorString(
				tc.diagName,
				tc.message,
				tc.fix,
				tc.hint,
				tc.toolOutput,
			)
			if err.Error() != want {
				t.Fatalf("Error() = %q, want %q", err.Error(), want)
			}
		})
	}
}
