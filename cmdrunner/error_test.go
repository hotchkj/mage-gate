// Vision: DiagnosticError formatting, stable field layout, and errors.Is/As wrapping contracts.
package cmdrunner_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
)

var (
	errTestCause    = errors.New("underlying failure")
	errTestPlain    = errors.New("go test failed with exit code 1")
	errTestSentinel = errors.New("sentinel cause")
)

const (
	testNameValue   = "test-name"
	testFailedValue = "test failed"
	fixTheTestValue = "fix the test"
	checkLogsValue  = "check logs"
	toolOutputValue = "tool output"
)

func TestDiagnosticFormatOrdering(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError("test-name", "test failed", "fix the test", "check logs", nil)
	want := cmdtest.ExpectedDiagnosticErrorString("test-name", "test failed", "fix the test", "check logs", "")
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDiagnosticFormatPreservesExistingDiagnostic(t *testing.T) {
	t.Parallel()

	existingErr := cmdrunner.NewDiagnosticError(
		"existing-name", "existing failed", "fix existing", "existing details", nil,
	)

	wrappedErr := cmdrunner.WrapDiagnostic("new-name", existingErr)

	if !errors.Is(wrappedErr, existingErr) {
		t.Error("expected WrapDiagnostic to preserve existing diagnostic format")
	}

	var de *cmdrunner.DiagnosticError
	if !errors.As(wrappedErr, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", wrappedErr)
	}
	if de.Message() != "existing failed" {
		t.Errorf("expected original error message, got: %s", de.Message())
	}
}

func TestWrapDiagnosticUnwrapSentinel(t *testing.T) {
	t.Parallel()

	wrapped := cmdrunner.WrapDiagnostic("outer", fmt.Errorf("wrap: %w", errTestSentinel))
	if !errors.Is(wrapped, errTestSentinel) {
		t.Fatal("expected errors.Is through wrapped DiagnosticError")
	}
}

func TestDiagnosticFormatNilError(t *testing.T) {
	t.Parallel()

	wrappedErr := cmdrunner.WrapDiagnostic("test-name", nil)
	if wrappedErr != nil {
		t.Error("expected WrapDiagnostic to return nil for nil error")
	}
}

func TestNewDiagnosticError_ToolOutputPopulated(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(
		testNameValue,
		testFailedValue,
		fixTheTestValue,
		checkLogsValue,
		&cmdrunner.DiagnosticOptions{ToolOutput: "tool output line 1\ntool output line 2"},
	)

	var de *cmdrunner.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", err)
	}
	if de.Name() != testNameValue {
		t.Errorf("Name: got %q, want %q", de.Name(), testNameValue)
	}
	if de.Message() != testFailedValue {
		t.Errorf("Message: got %q, want %q", de.Message(), testFailedValue)
	}
	if de.Fix() != fixTheTestValue {
		t.Errorf("Fix: got %q, want %q", de.Fix(), fixTheTestValue)
	}
	if de.Hint() != checkLogsValue {
		t.Errorf("Hint: got %q, want %q", de.Hint(), checkLogsValue)
	}
	if de.ToolOutput() != "tool output line 1\ntool output line 2" {
		t.Errorf("ToolOutput: got %q, want %q", de.ToolOutput(), "tool output line 1\ntool output line 2")
	}
}

func TestNewDiagnosticError_EmptyOptions(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(
		testNameValue,
		testFailedValue,
		fixTheTestValue,
		checkLogsValue,
		&cmdrunner.DiagnosticOptions{},
	)

	var de *cmdrunner.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", err)
	}
	if de.Name() != testNameValue {
		t.Errorf("Name: got %q, want %q", de.Name(), testNameValue)
	}
	if de.Message() != testFailedValue {
		t.Errorf("Message: got %q, want %q", de.Message(), testFailedValue)
	}
	if de.Fix() != fixTheTestValue {
		t.Errorf("Fix: got %q, want %q", de.Fix(), fixTheTestValue)
	}
	if de.Hint() != checkLogsValue {
		t.Errorf("Hint: got %q, want %q", de.Hint(), checkLogsValue)
	}
}

func TestDiagnosticErrorFields(t *testing.T) {
	t.Parallel()

	diagErr := cmdrunner.NewDiagnosticError(
		testNameValue,
		testFailedValue,
		fixTheTestValue,
		checkLogsValue,
		&cmdrunner.DiagnosticOptions{ToolOutput: toolOutputValue},
	)

	if errors.Unwrap(diagErr) != nil {
		t.Fatalf("expected no cause, got %v", errors.Unwrap(diagErr))
	}
	var de *cmdrunner.DiagnosticError
	if !errors.As(diagErr, &de) {
		t.Fatalf("expected *DiagnosticError")
	}
	if de.Name() != testNameValue {
		t.Errorf("expected name to be %q, got: %s", testNameValue, de.Name())
	}
	if de.Message() != testFailedValue {
		t.Errorf("expected message to be %q, got: %s", testFailedValue, de.Message())
	}
	if de.Fix() != fixTheTestValue {
		t.Errorf("expected fix to be %q, got: %s", fixTheTestValue, de.Fix())
	}
	if de.Hint() != checkLogsValue {
		t.Errorf("expected hint to be %q, got: %s", checkLogsValue, de.Hint())
	}
	if de.ToolOutput() != toolOutputValue {
		t.Errorf("expected toolOutput to be %q, got: %s", toolOutputValue, de.ToolOutput())
	}
}

func TestDiagnosticErrorString(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(
		testNameValue,
		testFailedValue,
		fixTheTestValue,
		checkLogsValue,
		&cmdrunner.DiagnosticOptions{ToolOutput: toolOutputValue},
	)

	var de *cmdrunner.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", err)
	}
	if de.Name() != testNameValue {
		t.Errorf("Name: got %q, want %q", de.Name(), testNameValue)
	}
	if de.Message() != testFailedValue {
		t.Errorf("Message: got %q, want %q", de.Message(), testFailedValue)
	}
	if de.Fix() != fixTheTestValue {
		t.Errorf("Fix: got %q, want %q", de.Fix(), fixTheTestValue)
	}
	if de.Hint() != checkLogsValue {
		t.Errorf("Hint: got %q, want %q", de.Hint(), checkLogsValue)
	}
	if de.ToolOutput() != toolOutputValue {
		t.Errorf("ToolOutput: got %q, want %q", de.ToolOutput(), toolOutputValue)
	}
}

func TestDiagnosticErrorEmptyFields(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(testNameValue, testFailedValue, "", "", nil)
	want := cmdtest.ExpectedDiagnosticErrorString(testNameValue, testFailedValue, "", "", "")
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDiagnosticErrorWithOnlyFix(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(testNameValue, testFailedValue, fixTheTestValue, "", nil)
	want := cmdtest.ExpectedDiagnosticErrorString(testNameValue, testFailedValue, fixTheTestValue, "", "")
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDiagnosticErrorWithOnlyHint(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(testNameValue, testFailedValue, "", checkLogsValue, nil)
	want := cmdtest.ExpectedDiagnosticErrorString(testNameValue, testFailedValue, "", checkLogsValue, "")
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDiagnosticErrorWithToolOutputOnly(t *testing.T) {
	t.Parallel()

	err := cmdrunner.NewDiagnosticError(
		testNameValue, testFailedValue, "", "",
		&cmdrunner.DiagnosticOptions{ToolOutput: toolOutputValue},
	)

	want := cmdtest.ExpectedDiagnosticErrorString(testNameValue, testFailedValue, "", "", toolOutputValue)
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDiagnosticErrorEmptyNameOmitsBrackets(t *testing.T) {
	t.Parallel()

	diagErr := cmdrunner.NewDiagnosticError("", "something broke", "do this", "see that", nil)
	want := cmdtest.ExpectedDiagnosticErrorString("", "something broke", "do this", "see that", "")
	if diagErr.Error() != want {
		t.Fatalf("Error() = %q, want %q", diagErr.Error(), want)
	}
}

func TestNewDiagnosticError_CauseUnwraps(t *testing.T) {
	t.Parallel()

	diagErr := cmdrunner.NewDiagnosticError(
		"s", "msg", "fix", "hint",
		&cmdrunner.DiagnosticOptions{ToolOutput: "out", Cause: errTestCause},
	)

	var de *cmdrunner.DiagnosticError
	if !errors.As(diagErr, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", diagErr)
	}
	unwrapped := errors.Unwrap(diagErr)
	if !errors.Is(unwrapped, errTestCause) {
		t.Fatalf("expected Unwrap() to return cause %v, got %v", errTestCause, unwrapped)
	}
	if !errors.Is(diagErr, errTestCause) {
		t.Fatal("expected errors.Is to reach cause through DiagnosticError")
	}
}

func TestWrapDiagnosticGeneratedContent(t *testing.T) {
	t.Parallel()

	wrapped := cmdrunner.WrapDiagnostic("test", errTestPlain)

	var de *cmdrunner.DiagnosticError
	if !errors.As(wrapped, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", wrapped)
	}
	if de.Name() != "test" {
		t.Errorf("expected name 'test', got %q", de.Name())
	}
	// WrapDiagnostic generates content; just verify it's not empty
	if de.Message() == "" {
		t.Errorf("expected non-empty generated message")
	}
	if de.Fix() == "" {
		t.Errorf("expected non-empty generated fix")
	}
	if de.Hint() == "" {
		t.Errorf("expected non-empty generated hint")
	}
	// Verify original error is reachable
	if !errors.Is(wrapped, errTestPlain) {
		t.Fatal("expected errors.Is to reach original error through wrapped DiagnosticError")
	}
}

func TestWrapDiagnosticPreservesIdentity(t *testing.T) {
	t.Parallel()

	original := cmdrunner.NewDiagnosticError("lint", "lint failed", "fix lint", "run linter", nil)
	wrapped := cmdrunner.WrapDiagnostic("outer", original)

	if !errors.Is(wrapped, original) {
		t.Fatal("expected WrapDiagnostic to return the same *DiagnosticError, not a new wrapper")
	}
}
