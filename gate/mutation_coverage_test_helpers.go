// Vision: Helper functions for mutation coverage tests to reduce file length.
package gate

import (
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const (
	wantWorstFileRowsCount = 2
	wantSummaryTotal       = 11
	wantSummaryNotCovered  = 5
)

func verifyMutationCoverageDiagnosticWithMsg(
	t *testing.T, de *DiagnosticError, wantFix, wantHint string, result *gatecheck.MutationCoverageResult,
) {
	t.Helper()

	verifyMutationCoverageFixWithMsg(t, de.Fix(), wantFix)
	verifyMutationCoverageHintWithMsg(t, de.Hint(), wantHint)
	wantToolOutput := gatecheck.FormatMutationCoverageResultRows(result, gatecheck.MaxWorstFileRows)
	if de.ToolOutput() != wantToolOutput {
		t.Fatalf("ToolOutput mismatch:\n  got:  %q\n  want: %q", de.ToolOutput(), wantToolOutput)
	}
}

func verifyMutationCoverageFixWithMsg(t *testing.T, fix, wantFix string) {
	t.Helper()

	if fix == "" {
		t.Error("expected non-empty Fix")
	}
	if fix != wantFix {
		t.Errorf("Fix mismatch:\n  got:  %q\n  want: %q", fix, wantFix)
	}
}

func verifyMutationCoverageHintWithMsg(t *testing.T, hint, wantHint string) {
	t.Helper()

	if hint == "" {
		t.Error("expected non-empty Hint")
	}
	if hint != wantHint {
		t.Errorf("Hint mismatch:\n  got:  %q\n  want: %q", hint, wantHint)
	}
}

func hasExactLine(s, wantLine string) bool {
	for _, line := range splitLines(s) {
		if line == wantLine {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func verifyMutationCoverageResult(t *testing.T, result *gatecheck.MutationCoverageResult) {
	t.Helper()

	if len(result.WorstFileRows) == 0 {
		t.Fatal("expected worst-file rows")
	}
	if len(result.WorstFileRows) != wantWorstFileRowsCount {
		t.Errorf("expected %d worst-file rows, got %d", wantWorstFileRowsCount, len(result.WorstFileRows))
	}
	verifyWorstCoverageRow(t, result.WorstFileRows, 0, "pkg/ugly.go", 0)
	verifyWorstCoverageRow(t, result.WorstFileRows, 1, "pkg/bad.go", -1)

	verifyMutationCoverageSummary(t, result.Summary)
}

func verifyWorstCoverageRow(
	t *testing.T,
	rows []gatecheck.MutationCoverageRow,
	idx int,
	wantFile string,
	wantPercent float64,
) {
	t.Helper()

	if len(rows) <= idx {
		t.Fatalf("expected row index %d, got %d row(s)", idx, len(rows))
	}
	if rows[idx].File != wantFile {
		t.Errorf("row %d should be %s, got %q", idx, wantFile, rows[idx].File)
	}
	if wantPercent >= 0 && rows[idx].Percent != wantPercent {
		t.Errorf("row %d percent should be %.1f, got %f", idx, wantPercent, rows[idx].Percent)
	}
}

func verifyMutationCoverageSummary(t *testing.T, summary struct {
	Percent        float64
	MinPercent     int
	Covered        int
	Total          int
	NotCovered     int
	ThresholdError error
},
) {
	t.Helper()

	const wantPercent = 54.54545454545455
	if summary.Percent < wantPercent-0.0001 || summary.Percent > wantPercent+0.0001 {
		t.Errorf("summary percent mismatch: got %f, want ~%f", summary.Percent, wantPercent)
	}
	if summary.Total != wantSummaryTotal {
		t.Errorf("summary total mismatch: got %d, want %d", summary.Total, wantSummaryTotal)
	}
	if summary.NotCovered != wantSummaryNotCovered {
		t.Errorf("summary notCovered mismatch: got %d, want %d", summary.NotCovered, wantSummaryNotCovered)
	}
}
