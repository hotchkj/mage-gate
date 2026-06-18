// Vision: test2json event decoding and per-test duration enforcement against max-seconds thresholds.
package gatecheck

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const pkgFooTestBar = "pkg/foo.TestBar"

// testRunEvents produces a minimal go test -json sequence for a tested package.
// The package-level pass intentionally carries a separate elapsed value so tests can
// prove duration enforcement ignores package wall-clock.
func testRunEvents(pkg, testName string, testElapsed, pkgElapsed float64) string {
	run := fmt.Sprintf(`{"Action":"run","Package":%q,"Test":%q}`, pkg, testName)
	testPass := fmt.Sprintf(
		`{"Action":"pass","Package":%q,"Test":%q,"Elapsed":%s}`,
		pkg, testName, formatFloat(testElapsed),
	)
	pkgPass := fmt.Sprintf(
		`{"Action":"pass","Package":%q,"Elapsed":%s}`,
		pkg, formatFloat(pkgElapsed),
	)
	return run + "\n" + testPass + "\n" + pkgPass
}

func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}

func TestDuration_Passed(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader(testRunEvents("pkg/foo", "TestBar", 0.1, 0.5))
	result, err := Duration(reader, 1.0)
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected passed, got failed")
	}
}

func TestDuration_Failed(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader(testRunEvents("pkg/foo", "TestBar", 2.0, 2.0))
	result, err := Duration(reader, 1.0)
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed, got passed")
	}
	if len(result.Tests) != 1 || result.Tests[0].Name() != pkgFooTestBar {
		t.Fatalf("Tests = %#v, want pkg/foo.TestBar offender", result.Tests)
	}
}

func TestDuration_PackageWallClockIgnored(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader(testRunEvents("pkg/foo", "TestFast", 0.1, 2.0))
	result, err := Duration(reader, 1.0)
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected package wall-clock to be ignored, got failed result %#v", result)
	}
}

func TestDuration_SubtestsAndParentCheckedIndependently(t *testing.T) {
	t.Parallel()
	input := testRunEvents("pkg/foo", "TestGrouped/subcase", 0.5, 1.2) + "\n" +
		testRunEvents("pkg/foo", "TestGrouped", 1.2, 1.2)
	result, err := Duration(strings.NewReader(input), 1.0)
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected parent test completion over threshold to fail")
	}
	report := FormatDurationReport(result)
	want := "Tests with duration > 1.000s (required max):\n  1.200s  pkg/foo.TestGrouped\n"
	if report != want {
		t.Fatalf("FormatDurationReport() = %q, want %q", report, want)
	}
}

func TestDuration_RejectsNonPositiveMaxSeconds(t *testing.T) {
	t.Parallel()
	input := testRunEvents("pkg/foo", "TestBar", 0.1, 0.5)
	for _, max := range []float64{0, -1} {
		reader := strings.NewReader(input)
		_, err := Duration(reader, max)
		if err == nil {
			t.Fatalf("Duration(..., %v): expected error", max)
		}
		if !errors.Is(err, errInvalidMaxDurationSeconds) {
			t.Fatalf("Duration(..., %v): want errInvalidMaxDurationSeconds, got %v", max, err)
		}
	}
}

func TestIsTestCompletionEvent_RequiresNonEmptyTest(t *testing.T) {
	t.Parallel()
	if isTestCompletionEvent(TestEvent{Action: "run", Package: "pkg/p", Test: "TestX"}) {
		t.Fatal("run action must not count as completion")
	}
	if isTestCompletionEvent(TestEvent{Action: "pass", Package: "pkg/p", Test: ""}) {
		t.Fatal("package-level pass must not count as test completion")
	}
	if !isTestCompletionEvent(TestEvent{Action: "pass", Package: "pkg/p", Test: "TestX"}) {
		t.Fatal("expected pass with test name to count")
	}
	if !isTestCompletionEvent(TestEvent{Action: "fail", Package: "pkg/p", Test: "TestX"}) {
		t.Fatal("expected fail with test name to count")
	}
}

func TestProcessTestLine_KeepsMaxTestElapsed(t *testing.T) {
	t.Parallel()
	completions := map[string]TestDuration{}
	lines := []string{
		`{"Action":"pass","Package":"pkg/a","Test":"TestA","Elapsed":0.01}`,
		`{"Action":"pass","Package":"pkg/a","Test":"TestA","Elapsed":0.9}`,
		`{"Action":"pass","Package":"pkg/a","Test":"TestA","Elapsed":0.1}`,
	}
	for _, line := range lines {
		if err := processTestLine(line, completions); err != nil {
			t.Fatalf("processTestLine: %v", err)
		}
	}
	got := completions[testDurationKey("pkg/a", "TestA")]
	if got.Elapsed != 0.9 {
		t.Fatalf("expected max elapsed 0.9, got %v", got.Elapsed)
	}
}

func TestParseTestEvents_PackageLevelFailIgnored(t *testing.T) {
	t.Parallel()
	input := testRunEvents("pkg/foo", "TestBar", 0.05, 0.7) + "\n" +
		`{"Action":"fail","Package":"pkg/foo","Elapsed":0.9}`
	tests, err := ParseTestEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTestEvents() error = %v", err)
	}
	if len(tests) != 1 || tests[0].Name() != pkgFooTestBar || tests[0].Elapsed != 0.05 {
		t.Fatalf("ParseTestEvents() = %#v, want pkg/foo.TestBar at 0.05", tests)
	}
}

func TestCollectDurationOffenders_AtMaxIsNotOffender(t *testing.T) {
	t.Parallel()
	off := collectDurationOffenders([]TestDuration{{Package: "pkg/a", Test: "TestAt", Elapsed: 1.0}}, 1.0)
	if len(off) != 0 {
		t.Fatalf("expected no offenders at exactly max, got %#v", off)
	}
}

func TestCollectDurationOffenders_ClearlyAboveMaxIsOffender(t *testing.T) {
	t.Parallel()
	off := collectDurationOffenders([]TestDuration{{Package: "pkg/a", Test: "TestOver", Elapsed: 1.5}}, 1.0)
	if len(off) != 1 || off[0].Name() != "pkg/a.TestOver" {
		t.Fatalf("expected one offender, got %#v", off)
	}
}

func TestFormatDurationReport_SkipsExactlyAtMax(t *testing.T) {
	t.Parallel()
	result := DurationResult{
		Passed:     false,
		MaxSeconds: 1.0,
		Tests: []TestDuration{
			{Package: "pkg/at", Test: "TestAt", Elapsed: 1.0},
			{Package: "pkg/over", Test: "TestOver", Elapsed: 1.5},
		},
	}
	report := FormatDurationReport(result)
	want := "Tests with duration > 1.000s (required max):\n  1.500s  pkg/over.TestOver\n"
	if report != want {
		t.Fatalf("FormatDurationReport() = %q, want %q", report, want)
	}
}

func TestParseTestEvents_MultiplePackages(t *testing.T) {
	t.Parallel()
	input := testRunEvents("pkg/foo", "TestA", 0.1, 0.5) + "\n" +
		testRunEvents("pkg/bar", "TestB", 0.2, 0.3) + "\n" +
		testRunEvents("pkg/foo", "TestC", 0.3, 0.6)
	tests, err := ParseTestEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTestEvents() error = %v", err)
	}
	wantNames := []string{"pkg/bar.TestB", "pkg/foo.TestA", "pkg/foo.TestC"}
	if len(tests) != len(wantNames) {
		t.Fatalf("got %d tests, want %d: %#v", len(tests), len(wantNames), tests)
	}
	for i, wantName := range wantNames {
		if tests[i].Name() != wantName {
			t.Fatalf("test %d name = %q, want %q (all tests %#v)", i, tests[i].Name(), wantName, tests)
		}
	}
}

func TestParseTestEvents_CompileOnlyPackage(t *testing.T) {
	t.Parallel()
	input := testRunEvents("pkg/tested", "TestFoo", 0.1, 0.5) + "\n" +
		`{"Action":"pass","Package":"pkg/compile-only","Elapsed":30.0}`
	tests, err := ParseTestEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTestEvents() error = %v", err)
	}
	if len(tests) != 1 || tests[0].Name() != "pkg/tested.TestFoo" || tests[0].Elapsed != 0.1 {
		t.Fatalf("ParseTestEvents() = %#v, want only pkg/tested.TestFoo at 0.1", tests)
	}
}

func TestParseTestEvents_Empty(t *testing.T) {
	t.Parallel()
	_, err := ParseTestEvents(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errNoTestCompletions) {
		t.Fatalf("expected errNoTestCompletions, got %v", err)
	}
}

func TestParseTestEvents_TestCompletionWithoutRun(t *testing.T) {
	t.Parallel()
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"pkg/foo","Test":"TestBar","Elapsed":0.1}`
	tests, err := ParseTestEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTestEvents() error = %v", err)
	}
	if len(tests) != 1 || tests[0].Name() != pkgFooTestBar || tests[0].Elapsed != 0.1 {
		t.Fatalf("ParseTestEvents() = %#v, want pkg/foo.TestBar at 0.1", tests)
	}
}

func TestFormatDurationReport_Passed(t *testing.T) {
	t.Parallel()
	result := DurationResult{
		Passed:     true,
		MaxSeconds: 5.0,
	}
	report := FormatDurationReport(result)
	want := "Test durations OK (all <= 5.000s)"
	if report != want {
		t.Fatalf("FormatDurationReport() = %q, want %q", report, want)
	}
}

func TestFormatDurationReport_Failed(t *testing.T) {
	t.Parallel()
	result := DurationResult{
		Passed:     false,
		MaxSeconds: 1.5,
		Tests: []TestDuration{
			{Package: "pkg/slow", Test: "TestSlow", Elapsed: 3.0},
		},
	}
	report := FormatDurationReport(result)
	want := "Tests with duration > 1.500s (required max):\n  3.000s  pkg/slow.TestSlow\n"
	if report != want {
		t.Fatalf("FormatDurationReport() = %q, want %q", report, want)
	}
}

func TestDurationFromTests_Passed(t *testing.T) {
	t.Parallel()
	result, err := DurationFromTests([]TestDuration{{Package: "pkg/a", Test: "TestA", Elapsed: 0.5}}, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Fatal("expected passed")
	}
}

func TestDurationFromTests_Failed(t *testing.T) {
	t.Parallel()
	result, err := DurationFromTests([]TestDuration{{Package: "pkg/a", Test: "TestA", Elapsed: 2.0}}, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed")
	}
}

func TestDurationFromTests_Empty(t *testing.T) {
	t.Parallel()
	_, err := DurationFromTests([]TestDuration{}, 1.0)
	if err == nil {
		t.Fatal("expected error for empty tests")
	}
	if !errors.Is(err, errNoTestCompletions) {
		t.Fatalf("expected errNoTestCompletions, got %v", err)
	}
}

func TestDurationFromTests_RejectsNonPositiveMaxSeconds(t *testing.T) {
	t.Parallel()
	_, err := DurationFromTests([]TestDuration{{Package: "pkg/a", Test: "TestA", Elapsed: 0.5}}, 0)
	if err == nil {
		t.Fatal("expected error for maxSeconds=0")
	}
	if !errors.Is(err, errInvalidMaxDurationSeconds) {
		t.Fatalf("want errInvalidMaxDurationSeconds, got %v", err)
	}
}

func TestSortTestDurations_TieBreakByName(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "pkg/b", Test: "TestB", Elapsed: 1.0},
		{Package: "pkg/a", Test: "TestA", Elapsed: 1.0},
	}
	list := sortTestDurations(tests)
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Elapsed != 1.0 || list[1].Elapsed != 1.0 {
		t.Fatalf("unexpected ordering: %#v", list)
	}
	if list[0].Name() != "pkg/a.TestA" || list[1].Name() != "pkg/b.TestB" {
		t.Fatalf("expected lexicographic tie-break, got %#v", list)
	}
}

func TestSortTestDurations_ByElapsedDesc(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "pkg/fast", Test: "TestFast", Elapsed: 0.1},
		{Package: "pkg/slow", Test: "TestSlow", Elapsed: 3.0},
	}
	list := sortTestDurations(tests)
	if list[0].Name() != "pkg/slow.TestSlow" || list[0].Elapsed != 3.0 {
		t.Fatalf("expected slow first, got %#v", list[0])
	}
}

func TestSortTestDurations_OrderByElapsedThenName(t *testing.T) {
	t.Parallel()
	tests := []TestDuration{
		{Package: "pkg/mid", Test: "TestMid", Elapsed: 2.0},
		{Package: "pkg/lo", Test: "TestLow", Elapsed: 1.0},
		{Package: "pkg/hi", Test: "TestHigh", Elapsed: 3.0},
	}
	list := sortTestDurations(tests)
	if list[0].Name() != "pkg/hi.TestHigh" ||
		list[1].Name() != "pkg/mid.TestMid" ||
		list[2].Name() != "pkg/lo.TestLow" {
		t.Fatalf("unexpected order: %#v", list)
	}
}
