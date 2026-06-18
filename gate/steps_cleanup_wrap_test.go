// Vision: wrapHarnessCleanup—DiagnosticError in silent display, raw errors in verbose display for cleanup failures.
package gate

import (
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
)

// errWrapCleanupInner is a package-level sentinel for err113 (no dynamic errors.New in tests).
var errWrapCleanupInner = errors.New("wrap cleanup inner")

func TestWrapHarnessCleanupNil(t *testing.T) {
	t.Parallel()
	inner := cmdtest.NewFakeRunner()
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	if err := wrapHarnessCleanup("lint", runner, nil); err != nil {
		t.Fatalf("expected nil when cleanup succeeded, got %v", err)
	}
}

func TestWrapHarnessCleanupSilentDiagnostic(t *testing.T) {
	t.Parallel()
	inner := cmdtest.NewFakeRunner()
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	err := wrapHarnessCleanup("lint", runner, errWrapCleanupInner)
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if !errors.Is(err, errWrapCleanupInner) {
		t.Fatalf("expected errors.Is chain to cleanup sentinel, got %v", err)
	}
}

func TestWrapHarnessCleanupVerboseRaw(t *testing.T) {
	t.Parallel()
	inner := cmdtest.NewFakeRunner()
	runner := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)
	err := wrapHarnessCleanup("lint", runner, errWrapCleanupInner)
	var de *DiagnosticError
	if errors.As(err, &de) {
		t.Fatalf("expected raw error path in verbose display, got diagnostic: %v", err)
	}
	if !errors.Is(err, errWrapCleanupInner) {
		t.Fatalf("expected errors.Is chain to cleanup sentinel, got %v", err)
	}
}
