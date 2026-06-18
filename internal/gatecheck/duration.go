// Vision: Parse `go test -json` streams into per-test elapsed times for max-duration enforcement.
package gatecheck

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// TestEvent represents a go test -json event.
type TestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

// TestDuration represents one completed test and its elapsed duration.
type TestDuration struct {
	Package string
	Test    string
	Elapsed float64
}

// Name returns the report identity for a completed test.
func (d TestDuration) Name() string {
	return d.Package + "." + d.Test
}

// DurationResult holds the duration check result.
type DurationResult struct {
	Tests      []TestDuration
	MaxSeconds float64
	Passed     bool
}

// ParseTestEvents parses go test -json output and extracts per-test durations.
// Package-level completion events are ignored so package wall-clock, coverage
// instrumentation, and compile-only -coverpkg packages never affect duration checks.
func ParseTestEvents(reader io.Reader) ([]TestDuration, error) {
	completions := make(map[string]TestDuration)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if err := processTestLine(strings.TrimSpace(scanner.Text()), completions); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(completions) == 0 {
		return nil, errNoTestCompletions
	}
	return sortParsedTestDurations(completions), nil
}

func processTestLine(line string, completions map[string]TestDuration) error {
	if line == "" {
		return nil
	}
	var event TestEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return fmt.Errorf("invalid test2json line (expected JSON per line from go test -json): %w", err)
	}
	if !isTestCompletionEvent(event) {
		return nil
	}
	key := testDurationKey(event.Package, event.Test)
	existing, ok := completions[key]
	if !ok || event.Elapsed > existing.Elapsed {
		completions[key] = TestDuration{
			Package: event.Package,
			Test:    event.Test,
			Elapsed: event.Elapsed,
		}
	}
	return nil
}

func testDurationKey(pkg, test string) string {
	return pkg + "\x00" + test
}

func isTestCompletionEvent(event TestEvent) bool {
	return (event.Action == "pass" || event.Action == "fail") &&
		event.Package != "" &&
		event.Test != ""
}

func sortParsedTestDurations(completions map[string]TestDuration) []TestDuration {
	tests := make([]TestDuration, 0, len(completions))
	for _, test := range completions {
		tests = append(tests, test)
	}
	sort.Slice(tests, func(left, right int) bool {
		if tests[left].Package != tests[right].Package {
			return tests[left].Package < tests[right].Package
		}
		return tests[left].Test < tests[right].Test
	})
	return tests
}

// Duration checks test durations against the maximum.
// maxSeconds must be > 0 (physically impossible threshold otherwise; see design doc §6).
func Duration(reader io.Reader, maxSeconds float64) (DurationResult, error) {
	if maxSeconds <= 0 {
		return DurationResult{}, errInvalidMaxDurationSeconds
	}
	tests, err := ParseTestEvents(reader)
	if err != nil {
		return DurationResult{}, err
	}
	return DurationFromTests(tests, maxSeconds)
}

// DurationFromTests checks test durations against the maximum.
// maxSeconds must be > 0 (same constraint as [Duration]).
func DurationFromTests(tests []TestDuration, maxSeconds float64) (DurationResult, error) {
	if maxSeconds <= 0 {
		return DurationResult{}, errInvalidMaxDurationSeconds
	}
	if len(tests) == 0 {
		return DurationResult{}, errNoTestCompletions
	}
	list := sortTestDurations(tests)
	offenders := collectDurationOffenders(tests, maxSeconds)
	return DurationResult{
		Tests:      list,
		MaxSeconds: maxSeconds,
		Passed:     len(offenders) == 0,
	}, nil
}

func sortTestDurations(tests []TestDuration) []TestDuration {
	list := append([]TestDuration(nil), tests...)
	sort.Slice(list, func(left, right int) bool {
		if list[left].Elapsed != list[right].Elapsed {
			return list[left].Elapsed > list[right].Elapsed
		}
		return list[left].Name() < list[right].Name()
	})
	return list
}

func collectDurationOffenders(tests []TestDuration, maxSeconds float64) []TestDuration {
	offenders := make([]TestDuration, 0, len(tests))
	for _, test := range tests {
		if test.Elapsed > maxSeconds+1e-9 {
			offenders = append(offenders, test)
		}
	}
	return offenders
}

// FormatDurationReport formats the duration check result for output.
func FormatDurationReport(result DurationResult) string {
	if result.Passed {
		return fmt.Sprintf("Test durations OK (all <= %.3fs)", result.MaxSeconds)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tests with duration > %.3fs (required max):\n", result.MaxSeconds)
	for _, test := range result.Tests {
		if test.Elapsed > result.MaxSeconds+1e-9 {
			fmt.Fprintf(&sb, "  %.3fs  %s\n", test.Elapsed, test.Name())
		}
	}
	return sb.String()
}
