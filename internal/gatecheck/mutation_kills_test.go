package gatecheck

// Vision: Kill-rate math on decoded gremlins reports: multi-file rollups, thresholds, and malformed JSON handling.

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestParseMutationKillsReport_FilesSection(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "pkg/foo.go",
				"mutations": [
					{"status": "KILLED"},
					{"status": "LIVED"},
					{"status": "KILLED"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 2 {
		t.Fatalf("expected 2 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
	if math.Abs(check.KillRatePercent-66.66666666666666) > 0.001 {
		t.Fatalf("expected ~66.67%%, got %.2f%%", check.KillRatePercent)
	}
}

func TestParseMutationKillsReport_StatusNormalization_Lowercase(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "killed"},
					{"status": "lived"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
}

func TestParseMutationKillsReport_StatusNormalization_MixedCase(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "KiLLeD"},
					{"status": "NoT_CoVeReD"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalNotCovered != 1 {
		t.Fatalf("expected 1 not_covered, got %d", check.TotalNotCovered)
	}
}

func TestParseMutationKillsReport_StatusNormalization_Trimspace(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "  KILLED  "},
					{"status": "\tLIVED\n"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
}

func TestParseMutationKillsReport_StatusNormalization_SpacedGremlinsLabels(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "TIMED OUT"},
					{"status": "NOT COVERED"},
					{"status": "NOT VIABLE"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalTimedOut != 1 {
		t.Fatalf("expected 1 timed_out, got %d", check.TotalTimedOut)
	}
	if check.TotalNotCovered != 1 {
		t.Fatalf("expected 1 not_covered, got %d", check.TotalNotCovered)
	}
	if check.TotalNotViable != 1 {
		t.Fatalf("expected 1 not_viable, got %d", check.TotalNotViable)
	}
}

func TestParseMutationKillsReport_TimedOutDetailFromGremlinsJSON(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "foo.go",
				"mutations": [
					{"status": "TIMED_OUT", "type": "INVERT_LOOPCTRL", "line": 12, "column": 7}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(check.Files))
	}
	fs := check.Files[0]
	if fs.TimedOut != 1 || len(fs.TimedOutDetails) != 1 {
		t.Fatalf("want 1 timed out with 1 detail, got %+v", fs)
	}
	want := "INVERT_LOOPCTRL line 12 col 7"
	if fs.TimedOutDetails[0] != want {
		t.Fatalf("detail %q, want %q", fs.TimedOutDetails[0], want)
	}
}

func TestParseMutationKillsReport_AllStatusTypes(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "KILLED"},
					{"status": "LIVED"},
					{"status": "NOT_COVERED"},
					{"status": "TIMED_OUT"},
					{"status": "NOT_VIABLE"},
					{"status": "RUNNABLE"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
	if check.TotalNotCovered != 1 {
		t.Fatalf("expected 1 not_covered, got %d", check.TotalNotCovered)
	}
	if check.TotalTimedOut != 1 {
		t.Fatalf("expected 1 timed_out, got %d", check.TotalTimedOut)
	}
	if check.TotalNotViable != 1 {
		t.Fatalf("expected 1 not_viable, got %d", check.TotalNotViable)
	}
	if check.TotalRunnable != 1 {
		t.Fatalf("expected 1 runnable, got %d", check.TotalRunnable)
	}
}

func TestParseMutationKillsReport_FileStats(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "pkg/foo.go",
				"mutations": [
					{"status": "KILLED"},
					{"status": "KILLED"},
					{"status": "LIVED"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(check.Files))
	}
	if check.Files[0].File != "pkg/foo.go" {
		t.Fatalf("expected pkg/foo.go, got %s", check.Files[0].File)
	}
	if check.Files[0].Killed != 2 {
		t.Fatalf("expected 2 killed in file, got %d", check.Files[0].Killed)
	}
	if check.Files[0].Lived != 1 {
		t.Fatalf("expected 1 lived in file, got %d", check.Files[0].Lived)
	}
}

func TestParseMutationKillsReport_PackageStats(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "pkg/foo.go",
				"package": "github.com/test/pkg",
				"mutations": [
					{"status": "KILLED"},
					{"status": "LIVED"},
					{"status": "NOT_COVERED"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Packages) != 1 {
		t.Fatalf("expected 1 package entry, got %d", len(check.Packages))
	}
	if check.Packages[0].Package != "github.com/test/pkg" {
		t.Fatalf("expected github.com/test/pkg, got %s", check.Packages[0].Package)
	}
	if check.Packages[0].Killed != 1 {
		t.Fatalf("expected 1 killed, got %d", check.Packages[0].Killed)
	}
	if check.Packages[0].NotCovered != 1 {
		t.Fatalf("expected 1 not_covered, got %d", check.Packages[0].NotCovered)
	}
}

func TestParseMutationKillsReport_UnknownStatus_Error(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "UNKNOWN_STATUS"}
				]
			}
		]
	}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if !errors.Is(err, errUnknownStatus) {
		t.Fatalf("expected errUnknownStatus, got %v", err)
	}
}

func TestParseMutationKillsReport_EmptyStatus_Error(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": ""}
				]
			}
		]
	}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if !errors.Is(err, errEmptyStatus) {
		t.Fatalf("expected errEmptyStatus, got %v", err)
	}
}

func TestParseMutationKillsReport_MissingStatus_Error(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"no_status_field": true}
				]
			}
		]
	}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if !errors.Is(err, errMissingStatus) {
		t.Fatalf("expected errMissingStatus, got %v", err)
	}
}

func TestParseMutationKillsReport_InvalidJSON(t *testing.T) {
	t.Parallel()
	jsonData := `{invalid json}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseMutationKillsReport_EmptyJSON(t *testing.T) {
	t.Parallel()
	jsonData := `{}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 0 {
		t.Fatalf("expected 0 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 0 {
		t.Fatalf("expected 0 lived, got %d", check.TotalLived)
	}
}

func TestParseMutationKillsReport_FilesPrecedence(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "pkg/foo.go",
				"mutations": [{"status": "KILLED"}]
			}
		],
		"mutations": [
			{"file": "pkg/bar.go", "status": "LIVED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed from files, got %d", check.TotalKilled)
	}
	if check.TotalLived != 0 {
		t.Fatalf("expected 0 lived (mutations ignored), got %d", check.TotalLived)
	}
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file from files section, got %d", len(check.Files))
	}
	if check.Files[0].File != "pkg/foo.go" {
		t.Fatalf("expected file from files section, got %s", check.Files[0].File)
	}
}

func TestParseMutationKillsReport_UnknownFile(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(check.Files))
	}
	if check.Files[0].File != unknownFile {
		t.Fatalf("expected unknown file, got %s", check.Files[0].File)
	}
}

func TestParseMutationKillsReport_FlatMutations_MissingFileAndPackage(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED"},
			{"status": "LIVED"},
			{"status": "NOT_COVERED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)

	if check.TotalKilled != 1 {
		t.Errorf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Errorf("expected 1 lived, got %d", check.TotalLived)
	}
	if check.TotalNotCovered != 1 {
		t.Errorf("expected 1 not_covered, got %d", check.TotalNotCovered)
	}

	if len(check.Files) != 1 || check.Files[0].File != unknownFile {
		t.Errorf("expected 1 file with 'unknown' name, got %v", check.Files)
	}
	if len(check.Packages) != 1 || check.Packages[0].Package != unknownPackage {
		t.Errorf("expected 1 package with 'unknown' name, got %v", check.Packages)
	}
}

func TestParseMutationKillsReport_FlatMutationsSection(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "pkg/bar.go", "status": "KILLED"},
			{"file": "pkg/bar.go", "status": "LIVED"},
			{"file": "pkg/baz.go", "status": "KILLED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 2 {
		t.Fatalf("expected 2 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
}

func TestParseMutationKillsReport_FlatMutationsMissingStatus_Error(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "pkg/test.go"}
		]
	}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if !errors.Is(err, errMissingStatus) {
		t.Fatalf("expected errMissingStatus, got %v", err)
	}
}

func TestParseMutationKillsReport_FlatMutationsEmptyMutations(t *testing.T) {
	t.Parallel()
	jsonData := `{"mutations":[]}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 0 {
		t.Fatalf("expected 0 files for empty mutations, got %d", len(check.Files))
	}
}

func TestParseMutationKillsReport_FlatMutationsWithPackage(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "cmd/main.go", "package": "main", "status": "KILLED"},
			{"file": "cmd/main.go", "package": "main", "status": "LIVED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.TotalKilled != 1 {
		t.Fatalf("expected 1 killed, got %d", check.TotalKilled)
	}
	if len(check.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(check.Packages))
	}
	if check.Packages[0].Package != "main" {
		t.Fatalf("expected 'main', got %s", check.Packages[0].Package)
	}
}
