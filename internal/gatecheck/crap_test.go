// Vision: CRAP index vs configured max: boundary values, floating-point edges, and gatecheck-only math.
package gatecheck

import (
	"errors"
	"math"
	"strings"
	"testing"
)

const (
	testCoverFuncOutput = `github.com/hotchkj/mage-gate/internal/harness/config.go:10:	Validate		100.0%
total:	(statements)	100.0%`
	testConfigPath = "internal/harness/config.go"
	// Repeated gocyclo fixture line (complexity 5, 100% coverage pairing in testCoverFuncOutput).
	testGocycloFiveValidate = `5 harness Validate internal/harness/config.go:10:1`
)

func TestCalculateCrap_ExactValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		comp     int
		covPct   float64
		wantCrap float64
	}{
		{1, 100, 1},
		{2, 50, 2.5},
		{3, 0, 12},
		{4, 25, 4*4*(0.75*0.75*0.75) + 4},
	}
	for _, tc := range cases {
		got := calculateCrap(tc.comp, tc.covPct)
		if math.Abs(got-tc.wantCrap) > 1e-12 {
			t.Fatalf("calculateCrap(%d, %g) = %g, want %g", tc.comp, tc.covPct, got, tc.wantCrap)
		}
	}
}

func TestCrap_FunctionAtThresholdIsOffender(t *testing.T) {
	t.Parallel()
	// CRAP = 5 at 100% coverage; threshold inclusive (>=) must still flag the function.
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(testGocycloFiveValidate, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 5.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed when CRAP equals max threshold (inclusive boundary)")
	}
	if len(result.Offenders) != 1 {
		t.Fatalf("Offenders = %v, want 1 entry", result.Offenders)
	}
	if result.Offenders[0].Name != "Validate" || result.Offenders[0].Crap != 5.0 {
		t.Fatalf("Offenders[0] = %+v, want {Name:Validate Crap:5.0}", result.Offenders[0])
	}
}

func TestCrap_Passed(t *testing.T) {
	t.Parallel()
	// With complexity 5 and 100% coverage: CRAP = 5² × (1-1)³ + 5 = 0 + 5 = 5 < 8
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(testGocycloFiveValidate, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed, got failed with %d offenders", len(result.Offenders))
	}
}

func TestCrap_Failed(t *testing.T) {
	t.Parallel()
	gocycloOutput := `15 harness Validate /repo/internal/harness/config.go:10:1`
	coverFuncOutput := `github.com/hotchkj/mage-gate/internal/harness/config.go:10:	Validate		0.0%`
	result, err := Crap(gocycloOutput, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed, got passed")
	}
	if len(result.Offenders) != 1 {
		t.Fatalf("Offenders = %v, want 1 entry", result.Offenders)
	}
	if result.Offenders[0].Name != "Validate" || result.Offenders[0].Crap != 240.0 {
		t.Fatalf("Offenders[0] = %+v, want {Name:Validate Crap:240.0}", result.Offenders[0])
	}
}

// TestCrap_EmptyInput documents "no gocyclo / cover-func lines" as vacuous success: there are no
// functions to score, so there are no offenders. This is unrelated to ErrAllPackagesExcluded from
// docs/mage-gate-intent-and-design.md §11, which is raised when exclude configuration removes every
// package from a filtered list (see internal/harness step tests), not when upstream tools emit empty text.
func TestCrap_EmptyInput(t *testing.T) {
	t.Parallel()
	result, err := Crap("", "", "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected passed for empty input")
	}
}

func TestCrap_OffendersAreSorted(t *testing.T) {
	t.Parallel()

	gocycloOutput := strings.Join([]string{
		"2 pfn Alpha foo.go:2:1",
		"4 pfn Zeta foo.go:1:1",
		"2 pfn Beta foo.go:3:1",
	}, "\n")
	coverFuncOutput := strings.Join([]string{
		"foo.go:2:\tAlpha\t0.0%",
		"foo.go:1:\tZeta\t0.0%",
		"foo.go:3:\tBeta\t0.0%",
		"total:\t(statements)\t100.0%",
	}, "\n")
	result, err := Crap(gocycloOutput, coverFuncOutput, "", "/repo", 0.1, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if len(result.Offenders) != 3 {
		t.Fatalf("Offenders = %d, want %d", len(result.Offenders), 3)
	}

	got := make([]string, 0, len(result.Offenders))
	for _, offender := range result.Offenders {
		got = append(got, offender.Name)
	}
	want := []string{"Zeta", "Alpha", "Beta"}
	for i, wantName := range want {
		if got[i] != wantName {
			t.Fatalf("offender %d = %q, want %q", i, got[i], wantName)
		}
	}
}

func TestCrap_InvalidCoverFuncPercent(t *testing.T) {
	t.Parallel()
	// Matches cover -func line shape but coverage token is "." so ParseFloat fails.
	coverFuncOutput := `github.com/hotchkj/mage-gate/internal/harness/config.go:10:	Validate	.%`
	_, err := Crap(testGocycloFiveValidate, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err == nil {
		t.Fatal("expected error for invalid coverage percent token")
	}
	if !errors.Is(err, ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}

func TestCrap_MalformedGocycloLine(t *testing.T) {
	t.Parallel()
	gocycloOutput := `not-a-number harness Validate internal/harness/config.go:10:1`
	coverFuncOutput := testCoverFuncOutput
	_, err := Crap(gocycloOutput, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err == nil {
		t.Fatal("expected error for malformed gocyclo line")
	}
	if !errors.Is(err, ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}

func TestCrap_GocycloLineTooFewFields(t *testing.T) {
	t.Parallel()
	gocycloOutput := `5 pkg`
	coverFuncOutput := testCoverFuncOutput
	_, err := Crap(gocycloOutput, coverFuncOutput, "github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err == nil {
		t.Fatal("expected error for gocyclo line with too few fields")
	}
	if !errors.Is(err, ErrCrapFailed) {
		t.Fatalf("expected ErrCrapFailed, got %v", err)
	}
}

func TestNormalizeFilePath_WindowsAbsolutePath(t *testing.T) {
	t.Parallel()
	path := `C:\Users\dev\repo\internal\harness\config.go`
	moduleRoot := `C:\Users\dev\repo`
	expected := testConfigPath
	result := normalizeFilePath(path, moduleRoot)
	if result != expected {
		t.Errorf("normalizeFilePath() = %q, want %q", result, expected)
	}
}

func TestNormalizeFilePath_UnixAbsolutePath(t *testing.T) {
	t.Parallel()
	path := "/home/dev/repo/internal/harness/config.go"
	moduleRoot := "/home/dev/repo"
	expected := testConfigPath
	result := normalizeFilePath(path, moduleRoot)
	if result != expected {
		t.Errorf("normalizeFilePath() = %q, want %q", result, expected)
	}
}

func TestNormalizeFilePath_RelativePath(t *testing.T) {
	t.Parallel()
	path := `internal\harness\config.go`
	moduleRoot := ""
	expected := testConfigPath
	result := normalizeFilePath(path, moduleRoot)
	if result != expected {
		t.Errorf("normalizeFilePath() = %q, want %q", result, expected)
	}
}

func TestNormalizeFilePath_MixedSeparators(t *testing.T) {
	t.Parallel()
	path := `internal\harness/config.go`
	moduleRoot := ""
	expected := testConfigPath
	result := normalizeFilePath(path, moduleRoot)
	if result != expected {
		t.Errorf("normalizeFilePath() = %q, want %q", result, expected)
	}
}

// Module root prefix must not strip when the next rune is not a path separator (/repox vs /repo/...).
func TestNormalizeFilePath_PrefixMatchWithoutDirSeparator(t *testing.T) {
	t.Parallel()
	got := normalizeFilePath("/repox", "/repo")
	if got != "/repox" {
		t.Fatalf("normalizeFilePath() = %q, want %q", got, "/repox")
	}
}

func TestCrap_WindowsPaths(t *testing.T) {
	t.Parallel()
	gocycloOutput := `5 harness Validate C:\Users\dev\repo\internal\harness\config.go:10:1`
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(
		gocycloOutput,
		coverFuncOutput,
		"github.com/hotchkj/mage-gate",
		`C:\Users\dev\repo`,
		8.0,
		nil,
	)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed, got failed with %d offenders", len(result.Offenders))
	}
}

func TestCrap_UnixPaths(t *testing.T) {
	t.Parallel()
	gocycloOutput := `5 harness Validate /home/dev/repo/internal/harness/config.go:10:1`
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(gocycloOutput, coverFuncOutput,
		"github.com/hotchkj/mage-gate", "/home/dev/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed, got failed with %d offenders", len(result.Offenders))
	}
}

func TestCrap_TestFilePatternFilters(t *testing.T) {
	t.Parallel()
	// Prod + *_test.go in gocyclo; *_test.go must be filtered by patterns.
	gocycloOutput := testGocycloFiveValidate + "\n" +
		"12 harness_test TestBigSetup internal/harness/config_test.go:50:1"
	coverFuncOutput := testCoverFuncOutput
	patterns := []string{"*_test.go"}
	result, err := Crap(gocycloOutput, coverFuncOutput,
		"github.com/hotchkj/mage-gate", "/repo", 8.0, patterns)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed (test file filtered out), got %d offenders", len(result.Offenders))
	}
}

func TestCrap_TestFilePatternNoMatch(t *testing.T) {
	t.Parallel()
	gocycloOutput := "15 harness Validate /repo/internal/harness/config.go:10:1"
	coverFuncOutput := `github.com/hotchkj/mage-gate/internal/harness/config.go:10:	Validate		0.0%`
	patterns := []string{"*_integration.go"}
	result, err := Crap(gocycloOutput, coverFuncOutput,
		"github.com/hotchkj/mage-gate", "/repo", 8.0, patterns)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed — file not filtered")
	}
}

func TestCrap_ReceiverNormalization(t *testing.T) {
	t.Parallel()
	// gocyclo uses receiver-qualified names; cover profile uses bare func—normalization must align them.
	gocycloOutput := "5 harness (*Config).Validate internal/harness/config.go:10:1"
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(gocycloOutput, coverFuncOutput,
		"github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed (receiver normalized), got %d offenders", len(result.Offenders))
	}
}

func TestCrap_ValueReceiverNormalization(t *testing.T) {
	t.Parallel()
	gocycloOutput := "5 harness (Dependencies).Validate internal/harness/config.go:10:1"
	coverFuncOutput := testCoverFuncOutput
	result, err := Crap(gocycloOutput, coverFuncOutput,
		"github.com/hotchkj/mage-gate", "/repo", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed (value receiver normalized), got %d offenders", len(result.Offenders))
	}
}

func TestStripReceiver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"Validate", "Validate"},
		{"(*Config).Validate", "Validate"},
		{"(Dependencies).Validate", "Validate"},
		{"(*StepRunner).stepCrap", "stepCrap"},
		{".LeadingDot", "LeadingDot"},
	}
	for _, tt := range tests {
		if got := stripReceiver(tt.input); got != tt.want {
			t.Errorf("stripReceiver(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchesTestFilePattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filePath string
		patterns []string
		want     bool
	}{
		{"internal/harness/config_test.go", []string{"*_test.go"}, true},
		{"internal/harness/config.go", []string{"*_test.go"}, false},
		{`D:\repo\internal\harness\config_test.go`, []string{"*_test.go"}, true},
		{"config.go", nil, false},
		{"config.go", []string{}, false},
	}
	for _, tt := range tests {
		if got := matchesTestFilePattern(tt.filePath, tt.patterns); got != tt.want {
			t.Errorf("matchesTestFilePattern(%q, %v) = %v, want %v",
				tt.filePath, tt.patterns, got, tt.want)
		}
	}
}

func TestFormatCrapReport_Passed(t *testing.T) {
	t.Parallel()
	report := FormatCrapReport(CrapResult{Passed: true, MaxCrap: 8.0})
	want := "CRAP check passed (max 8.0)"
	if report != want {
		t.Fatalf("FormatCrapReport() = %q, want %q", report, want)
	}
}

// extractFilePath tests document gocyclo's trailing "path:line:col" token shape used when aligning
// complexity rows with cover -func keys (see docs/mage-gate-intent-and-design.md §10 QualityScope / CRAP).

func TestExtractFilePath_StandardGocycloToken(t *testing.T) {
	t.Parallel()
	got := extractFilePath("internal/pkg/a.go:42:3")
	if got != "internal/pkg/a.go" {
		t.Fatalf("extractFilePath() = %q", got)
	}
}

func TestExtractFilePath_FileNameContainsColonBeforeLineCol(t *testing.T) {
	t.Parallel()
	// Two reduction steps: strip trailing :12 then :1 from the synthetic "file" token.
	got := extractFilePath("foo:bar.go:12:1")
	if got != "foo:bar.go" {
		t.Fatalf("extractFilePath() = %q, want foo:bar.go", got)
	}
}

func TestExtractFilePath_DriveAbsoluteToken(t *testing.T) {
	t.Parallel()
	got := extractFilePath(`C:/repo/pkg/file.go:10:1`)
	if got != `C:/repo/pkg/file.go` {
		t.Fatalf("extractFilePath() = %q, want %q", got, `C:/repo/pkg/file.go`)
	}
}

func TestExtractFilePath_AbsoluteUnixToken(t *testing.T) {
	t.Parallel()
	got := extractFilePath(`/tmp/repo/pkg/file.go:10:1`)
	if got != `/tmp/repo/pkg/file.go` {
		t.Fatalf("extractFilePath() = %q, want %q", got, `/tmp/repo/pkg/file.go`)
	}
}

func TestExtractFilePath_MalformedPathLikePrefix(t *testing.T) {
	t.Parallel()
	got := extractFilePath(`part:with:colon`)
	if got != `part:with` {
		t.Fatalf("extractFilePath() = %q, want %q", got, `part:with`)
	}
}

func TestExtractFilePath_SingleColonSuffix(t *testing.T) {
	t.Parallel()
	got := extractFilePath("token:withcolon")
	if got != "token" {
		t.Fatalf("extractFilePath() = %q, want token", got)
	}
}

func TestExtractFilePath_LeadingColonUnchanged(t *testing.T) {
	t.Parallel()
	// Only ':' is at index 0, so the idx > 0 guard skips stripping entirely.
	raw := ":nocolonafter"
	got := extractFilePath(raw)
	if got != raw {
		t.Fatalf("extractFilePath() = %q, want unchanged %q", got, raw)
	}
}

func TestFormatCrapReport_Failed(t *testing.T) {
	t.Parallel()
	report := FormatCrapReport(CrapResult{
		Passed:  false,
		MaxCrap: 8.0,
		Offenders: []CrapOffender{
			{Path: "internal/a.go", Name: "F", Complexity: 10, Coverage: 0.0, Crap: 100.0},
		},
	})
	want := "CRAP check failed: 1 function(s) exceed threshold of 8.0\n" +
		"  internal/a.go:F - CRAP=100.0 (complexity=10, coverage=0.0%)\n"
	if report != want {
		t.Fatalf("FormatCrapReport() = %q, want %q", report, want)
	}
}
