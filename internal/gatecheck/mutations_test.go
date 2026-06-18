// Vision: Gremlins site counts: JSON shapes, threshold compares, and union rules with quality-scope excludes.
package gatecheck

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	testFileA  = "a.go"
	testFileB  = "b.go"
	testFileC  = "c.go"
	testFileHi = "hi.go"
	testFileLo = "lo.go"
	testFileM  = "m.go"
	testFileZ  = "z.go"
)

func TestMutationSites_ZeroMaxSitesIsError(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"files":[]}`)
	_, err := MutationSites(jsonData, 0)
	if !errors.Is(err, errInvalidMaxSites) {
		t.Fatalf("expected errInvalidMaxSites, got %v", err)
	}
}

func TestMutationSites_NegativeMaxSitesIsError(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"files":[]}`)
	_, err := MutationSites(jsonData, -5)
	if !errors.Is(err, errInvalidMaxSites) {
		t.Fatalf("expected errInvalidMaxSites, got %v", err)
	}
}

func TestMutationSites_Passed(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(
		`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"},{"status":"LIVED"}]}]}`,
	)
	assertMutationSitesPassed(t, jsonData, 50)
}

func TestMutationSites_Failed(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(
		`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"},{"status":"LIVED"},{"status":"KILLED"}]}]}`,
	)
	assertMutationSitesFailed(t, jsonData, 1)
}

func TestCountMutationsPerFile_FlatMutations(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"mutations":[{"file":"pkg/foo.go"},{"file":"pkg/foo.go"},{"file":"pkg/bar.go"}]}`)
	perFile := countMutations(t, jsonData)
	assertMutationCount(t, perFile, "pkg/foo.go", 2)
	assertMutationCount(t, perFile, "pkg/bar.go", 1)
}

func TestCountMutationsPerFile_Empty(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{}`)
	perFile := countMutations(t, jsonData)
	assertEmptyMutations(t, perFile)
}

func TestCountMutationsPerFile_FlatMutationsInvalidShape(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"mutations":{}}`)
	_, err := CountMutationsPerFile(jsonData)
	if err == nil {
		t.Fatal("expected error for mutations value that is not an array")
	}
	var ut *json.UnmarshalTypeError
	if !errors.As(err, &ut) {
		t.Fatalf("expected *json.UnmarshalTypeError, got %v", err)
	}
}

func TestCountMutationsPerFile_FlatMutationsUnknownFile(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"mutations":[{}]}`)
	perFile := countMutations(t, jsonData)
	assertMutationCount(t, perFile, unknownFile, 1)
}

func mutationTestData(rawJSON string) []byte {
	return []byte(rawJSON)
}

func assertMutationSitesPassed(t *testing.T, jsonData []byte, maxSites int) {
	t.Helper()
	result, err := MutationSites(jsonData, maxSites)
	if err != nil {
		t.Fatalf("MutationSites() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected passed, got failed")
	}
}

func assertMutationSitesFailed(t *testing.T, jsonData []byte, maxSites int) {
	t.Helper()
	result, err := MutationSites(jsonData, maxSites)
	if err != nil {
		t.Fatalf("MutationSites() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed, got passed")
	}
}

func countMutations(t *testing.T, jsonData []byte) map[string]int {
	t.Helper()
	perFile, err := CountMutationsPerFile(jsonData)
	if err != nil {
		t.Fatalf("CountMutationsPerFile() error = %v", err)
	}
	return perFile
}

func assertMutationCount(t *testing.T, perFile map[string]int, file string, expected int) {
	t.Helper()
	if perFile[file] != expected {
		t.Fatalf("expected %d, got %d", expected, perFile[file])
	}
}

func assertEmptyMutations(t *testing.T, perFile map[string]int) {
	t.Helper()
	if len(perFile) != 0 {
		t.Fatalf("expected 0, got %d", len(perFile))
	}
}

func TestFormatMutationReport_Passed(t *testing.T) {
	t.Parallel()
	report := FormatMutationReport(MutationResult{Passed: true, MaxSites: 10})
	want := "Mutation site counts OK (none above 10 per file)."
	if report != want {
		t.Fatalf("FormatMutationReport() = %q, want %q", report, want)
	}
}

func TestFormatMutationReport_Failed(t *testing.T) {
	t.Parallel()
	report := FormatMutationReport(MutationResult{
		Passed:   false,
		MaxSites: 1,
		PerFile: []MutationPair{
			{Path: "pkg/hot.go", Count: 5},
		},
	})
	var want strings.Builder
	want.WriteString(
		"Files with more than 1 mutation sites (rule: split by theme, move >=30% of code out):\n",
	)
	want.WriteString("     5  pkg/hot.go\n")
	if report != want.String() {
		t.Fatalf("FormatMutationReport() = %q, want %q", report, want.String())
	}
}

func TestCountMutationsPerFile_InvalidFilesArrayShape(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(`{"files":[{"file_name":true,"mutations":[]}]}`)
	_, err := CountMutationsPerFile(jsonData)
	if err == nil {
		t.Fatal("expected error when files[] entries do not match the typed shape")
	}
}

func TestPickFileName_Primary(t *testing.T) {
	t.Parallel()
	got := pickFileName("dir/a.go", "")
	if got != "dir/a.go" {
		t.Fatalf("expected primary, got %q", got)
	}
}

func TestPickFileName_Fallback(t *testing.T) {
	t.Parallel()
	got := pickFileName("", "dir/b.go")
	if got != "dir/b.go" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestPickFileName_Unknown(t *testing.T) {
	t.Parallel()
	got := pickFileName("", "")
	if got != unknownFile {
		t.Fatalf("expected unknown, got %q", got)
	}
}

func TestPickFileName_BackslashNormalized(t *testing.T) {
	t.Parallel()
	got := pickFileName(`win\dir\c.go`, "")
	if got != "win/dir/c.go" {
		t.Fatalf("expected forward slashes, got %q", got)
	}
}

func TestSortMutationCounts_TieBreakByPath(t *testing.T) {
	t.Parallel()
	list := sortMutationCounts(map[string]int{testFileB: 2, testFileA: 2})
	want := []MutationPair{
		{Path: testFileA, Count: 2},
		{Path: testFileB, Count: 2},
	}
	if !slices.Equal(list, want) {
		t.Fatalf("result = %#v, want %#v", list, want)
	}
}

func TestSortMutationCounts_ByCountDesc(t *testing.T) {
	t.Parallel()
	list := sortMutationCounts(map[string]int{testFileLo: 1, testFileHi: 9})
	want := []MutationPair{
		{Path: testFileHi, Count: 9},
		{Path: testFileLo, Count: 1},
	}
	if !slices.Equal(list, want) {
		t.Fatalf("result = %#v, want %#v", list, want)
	}
}

func TestSortMutationCounts_ThreeWayTieByPath(t *testing.T) {
	t.Parallel()
	list := sortMutationCounts(map[string]int{testFileZ: 2, testFileM: 2, testFileA: 2})
	want := []MutationPair{
		{Path: testFileA, Count: 2},
		{Path: testFileM, Count: 2},
		{Path: testFileZ, Count: 2},
	}
	if !slices.Equal(list, want) {
		t.Fatalf("result = %#v, want %#v", list, want)
	}
}

func TestSortMutationCounts_OrderByCountThenPath(t *testing.T) {
	t.Parallel()
	list := sortMutationCounts(map[string]int{testFileB: 1, testFileC: 3, testFileA: 2})
	want := []MutationPair{
		{Path: testFileC, Count: 3},
		{Path: testFileA, Count: 2},
		{Path: testFileB, Count: 1},
	}
	if !slices.Equal(list, want) {
		t.Fatalf("result = %#v, want %#v", list, want)
	}
}

func TestMutationSites_CountEqualsMaxPasses(t *testing.T) {
	t.Parallel()
	jsonData := mutationTestData(
		`{"files":[{"file_name":"pkg/foo.go","mutations":[{},{},{},{},{}]}]}`,
	)
	assertMutationSitesPassed(t, jsonData, 5)
}

func TestFormatMutationReport_OmitsFilesAtExactlyMax(t *testing.T) {
	t.Parallel()
	report := FormatMutationReport(MutationResult{
		Passed:   false,
		MaxSites: 2,
		PerFile: []MutationPair{
			{Path: "pkg/at_max.go", Count: 2},
			{Path: "pkg/over.go", Count: 3},
		},
	})
	want := "Files with more than 2 mutation sites (rule: split by theme, move >=30% of code out):\n" +
		"     3  pkg/over.go\n"
	if report != want {
		t.Fatalf("FormatMutationReport() = %q, want %q", report, want)
	}
}
