// Vision: Shared gremlins JSON fixtures — site counts and kill stats must stay aligned on the same decode path.
package gatecheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
)

func TestGremlinsMutationDocument_FilesLayout_SiteCountMatchesMutationEntries(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{"file_name": "pkg/a.go", "package": "example.com/mod/pkg", "mutations": [
				{"status": "KILLED"},
				{"status": "LIVED"}
			]},
			{"filename": "pkg/b.go", "mutations": [{"status": "NOT_COVERED"}]},
			{"file_name": "pkg/empty.go", "mutations": []}
		]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseGremlinsMutationRoot: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("countMutationSitesFromRoot: %v", err)
	}
	assertGremlinsSiteCount(t, perFile, "pkg/a.go", 2)
	assertGremlinsSiteCount(t, perFile, "pkg/b.go", 1)
	assertGremlinsSiteCount(t, perFile, "pkg/empty.go", 0)

	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("buildMutationKillsCheckFromRoot: %v", err)
	}
	assertGremlinsKillTotals(t, check, 1, 1, 1, 0, 0)
	if len(check.Files) != 3 {
		t.Fatalf("expected 3 file stats (files[] row with empty mutations still appears), got %d", len(check.Files))
	}
}

func TestGremlinsMutationDocument_FlatLayout_SiteCountAndKillsAgree(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "pkg/foo.go", "package": "example.com/foo", "status": "KILLED"},
			{"filename": "pkg/foo.go", "status": "LIVED"},
			{"file": "pkg/bar.go", "status": "TIMED OUT"}
		]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseGremlinsMutationRoot: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("countMutationSitesFromRoot: %v", err)
	}
	if perFile["pkg/foo.go"] != 2 {
		t.Fatalf("pkg/foo.go: want 2, got %d", perFile["pkg/foo.go"])
	}
	if perFile["pkg/bar.go"] != 1 {
		t.Fatalf("pkg/bar.go: want 1, got %d", perFile["pkg/bar.go"])
	}

	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("buildMutationKillsCheckFromRoot: %v", err)
	}
	if check.TotalKilled != 1 || check.TotalLived != 1 || check.TotalTimedOut != 1 {
		t.Fatalf("totals: killed=%d lived=%d timed_out=%d",
			check.TotalKilled, check.TotalLived, check.TotalTimedOut)
	}
}

func TestGremlinsMutationDocument_FilesPrecedence_IgnoresFlatWhenFilesUsable(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [{"file_name": "only.go", "mutations": [{"status": "KILLED"}]}],
		"mutations": [{"file": "ignored.go", "status": "LIVED"}]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("sites: %v", err)
	}
	if count, ok := perFile["only.go"]; ok && count != 1 {
		t.Fatalf("sites: only.go should have 1, got %d", count)
	}
	if _, ok := perFile["only.go"]; !ok {
		t.Fatalf("sites: only.go missing; keys present: %v", perFile)
	}
	if count, ok := perFile["ignored.go"]; ok {
		t.Fatalf("expected ignored.go absent from perFile, got count=%d", count)
	}
	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("kills: %v", err)
	}
	if check.TotalKilled != 1 || check.TotalLived != 0 {
		t.Fatalf("kills: killed=%d lived=%d", check.TotalKilled, check.TotalLived)
	}
}

func TestGremlinsMutationDocument_EmptyFilesArrayFallsBackToFlatMutations(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [],
		"mutations": [
			{"file": "pkg/x.go", "status": "KILLED"},
			{"file": "pkg/x.go", "status": "LIVED"}
		]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("sites: %v", err)
	}
	if perFile["pkg/x.go"] != 2 {
		t.Fatalf("expected 2 sites from flat mutations when files is empty, got %#v", perFile)
	}
	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("kills: %v", err)
	}
	if check.TotalKilled != 1 || check.TotalLived != 1 {
		t.Fatalf("kills: killed=%d lived=%d", check.TotalKilled, check.TotalLived)
	}
}

func TestGremlinsMutationDocument_FlatMutationEmptyFileFallsBackToFilename(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "", "filename": "pkg/from_filename.go", "status": "KILLED"}
		]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("sites: %v", err)
	}
	if perFile["pkg/from_filename.go"] != 1 {
		t.Fatalf("sites: want pkg/from_filename.go count 1, got %#v", perFile)
	}
	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("kills: %v", err)
	}
	if check.TotalKilled != 1 || len(check.Files) != 1 || check.Files[0].File != "pkg/from_filename.go" {
		t.Fatalf("kills: want 1 killed and one file row pkg/from_filename.go, got killed=%d files=%+v",
			check.TotalKilled, check.Files)
	}
}

func TestGremlinsMutationDocument_FlatMutationWhitespaceOnlyFileFallsBackToFilename(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"file": "   ", "filename": "pkg/ws_fallback.go", "status": "KILLED"}
		]
	}`
	root, err := parseGremlinsMutationRoot([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perFile, err := countMutationSitesFromRoot(root)
	if err != nil {
		t.Fatalf("sites: %v", err)
	}
	if perFile["pkg/ws_fallback.go"] != 1 {
		t.Fatalf("sites: %#v", perFile)
	}
	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		t.Fatalf("kills: %v", err)
	}
	if check.TotalKilled != 1 || len(check.Files) != 1 || check.Files[0].File != "pkg/ws_fallback.go" {
		t.Fatalf("kills: got files=%+v killed=%d", check.Files, check.TotalKilled)
	}
}

func TestGremlinsStructuralError_FilesNotArray(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"files":{},"mutations":[{"file":"x.go","status":"KILLED"}]}`,
		`{"files":1,"mutations":[]}`,
		`{"files":"nope","mutations":[]}`,
	}
	for i := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			raw := cases[i]
			root, err := parseGremlinsMutationRoot([]byte(raw))
			if err != nil {
				t.Fatalf("parse root: %v", err)
			}
			_, err = countMutationSitesFromRoot(root)
			if err == nil {
				t.Fatal("expected structural error")
			}
			if !errors.Is(err, errGremlinsFilesNotArray) {
				t.Fatalf("want %v, got %v", errGremlinsFilesNotArray, err)
			}
		})
	}
}

func TestGremlinsStructuralError_FilesEntryMutationsMissingOrInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"files":[{"file_name":"a.go"}]}`,
		`{"files":[{"file_name":"a.go","mutations":null}]}`,
		`{"files":[{"file_name":"a.go","mutations":{}}]}`,
		`{"files":[{"file_name":"a.go","mutations":1}]}`,
	}
	for i := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			raw := cases[i]
			_, err := CountMutationsPerFile([]byte(raw))
			if err == nil {
				t.Fatal("expected structural error")
			}
			if !errors.Is(err, errGremlinsFileMutationsNotArray) {
				t.Fatalf("want %v, got %v", errGremlinsFileMutationsNotArray, err)
			}
		})
	}
}

func TestGremlinsStructuralError_KillsFilesEntryMutationsMissing(t *testing.T) {
	t.Parallel()
	raw := `{"files":[{"file_name":"a.go"}]}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(raw)))
	if err == nil {
		t.Fatal("expected structural error")
	}
	if !errors.Is(err, errGremlinsFileMutationsNotArray) {
		t.Fatalf("want %v, got %v", errGremlinsFileMutationsNotArray, err)
	}
}

func TestGremlinsStructuralError_MutationsNotArray(t *testing.T) {
	t.Parallel()
	raw := `{"mutations":{}}`
	root, err := parseGremlinsMutationRoot([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, err = countMutationSitesFromRoot(root)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errGremlinsMutationsNotArray) {
		t.Fatalf("want %v, got %v", errGremlinsMutationsNotArray, err)
	}
	var ut *json.UnmarshalTypeError
	if !errors.As(err, &ut) {
		t.Fatalf("expected wrapped *json.UnmarshalTypeError, got %T: %v", err, err)
	}
}

func TestGremlinsLenient_FlatMutationNonStringFileBucketsToUnknown(t *testing.T) {
	t.Parallel()
	raw := `{"mutations":[{"file":1,"status":"KILLED"}]}`
	check, err := ParseMutationKillsReport(bytes.NewReader([]byte(raw)))
	if err != nil {
		t.Fatalf("expected success with non-string file bucketed to unknown, got %v", err)
	}
	if check.TotalKilled != 1 {
		t.Fatalf("want 1 killed, got %d", check.TotalKilled)
	}
	if len(check.Files) != 1 || check.Files[0].File != unknownFile {
		t.Fatalf("want file=unknown, got %+v", check.Files)
	}
}

func TestGremlinsLenient_FlatMutationNonStringFileSiteCount(t *testing.T) {
	t.Parallel()
	raw := `{"mutations":[{"file":1},{"file":"real.go"}]}`
	perFile, err := CountMutationsPerFile([]byte(raw))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	assertGremlinsSiteCount(t, perFile, unknownFile, 1)
	assertGremlinsSiteCount(t, perFile, "real.go", 1)
}

func TestGremlinsError_FilesNestedMutationNonStringStatus(t *testing.T) {
	t.Parallel()
	raw := `{"files":[{"file_name":"a.go","mutations":[{"status":true}]}]}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(raw)))
	if err == nil {
		t.Fatal("expected error for non-string status")
	}
	if !errors.Is(err, errMissingStatus) {
		t.Fatalf("want %v, got %v", errMissingStatus, err)
	}
}

func TestGremlinsError_FlatMutationNonStringStatus(t *testing.T) {
	t.Parallel()
	raw := `{"mutations":[{"file":"a.go","status":42}]}`
	_, err := ParseMutationKillsReport(bytes.NewReader([]byte(raw)))
	if err == nil {
		t.Fatal("expected error for non-string status")
	}
	if !errors.Is(err, errMissingStatus) {
		t.Fatalf("want %v, got %v", errMissingStatus, err)
	}
}

func assertGremlinsSiteCount(t *testing.T, m map[string]int, path string, want int) {
	t.Helper()
	if got := m[path]; got != want {
		t.Fatalf("%s: want %d sites, got %d", path, want, got)
	}
}

func assertGremlinsKillTotals(
	t *testing.T,
	killsCheck *MutationKillsCheck,
	killed, lived, notCovered, timedOut, notViable int,
) {
	t.Helper()
	if killsCheck.TotalKilled != killed {
		t.Errorf("TotalKilled = %d, want %d", killsCheck.TotalKilled, killed)
	}
	if killsCheck.TotalLived != lived {
		t.Errorf("TotalLived = %d, want %d", killsCheck.TotalLived, lived)
	}
	if killsCheck.TotalNotCovered != notCovered {
		t.Errorf("TotalNotCovered = %d, want %d", killsCheck.TotalNotCovered, notCovered)
	}
	if killsCheck.TotalTimedOut != timedOut {
		t.Errorf("TotalTimedOut = %d, want %d", killsCheck.TotalTimedOut, timedOut)
	}
	if killsCheck.TotalNotViable != notViable {
		t.Errorf("TotalNotViable = %d, want %d", killsCheck.TotalNotViable, notViable)
	}
}
