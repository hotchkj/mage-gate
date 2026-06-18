package gatecheck

import (
	"errors"
	"testing"
)

const mutationKillsSampleFilesJSON = `{
	"files": [
		{
			"file_name": "pkg/a.go",
			"package": "example.com/mod/pkg",
			"mutations": [
				{"status": "KILLED"},
				{"status": "KILLED"}
			]
		}
	]
}`

func TestMutationKills_ExportedMinRateZero(t *testing.T) {
	t.Parallel()
	res, err := MutationKills([]byte(mutationKillsSampleFilesJSON), 0)
	if err != nil {
		t.Fatalf("MutationKills: %v", err)
	}
	if res.Check == nil {
		t.Fatal("expected Check")
	}
	if !res.Passed {
		t.Fatal("expected pass with min 0")
	}
}

func TestMutationKills_ExportedMinRateSucceeds(t *testing.T) {
	t.Parallel()
	res, err := MutationKills([]byte(mutationKillsSampleFilesJSON), 100)
	if err != nil {
		t.Fatalf("MutationKills: %v", err)
	}
	if !res.Passed {
		t.Fatal("expected pass at 100% kill rate")
	}
}

func TestMutationKills_ExportedMinRateFails(t *testing.T) {
	t.Parallel()
	low := `{
		"files": [
			{
				"file_name": "pkg/a.go",
				"package": "p",
				"mutations": [
					{"status": "KILLED"},
					{"status": "LIVED"}
				]
			}
		]
	}`
	res, err := MutationKills([]byte(low), 100)
	if err != nil {
		t.Fatalf("MutationKills: %v", err)
	}
	if res.Passed {
		t.Fatal("expected fail when min kill rate 100% but LIVED present")
	}
}

func TestMutationKills_ExportedInvalidMinRate(t *testing.T) {
	t.Parallel()
	_, err := MutationKills([]byte(mutationKillsSampleFilesJSON), -1)
	if !errors.Is(err, errInvalidMinKillRate) {
		t.Fatalf("expected errInvalidMinKillRate, got %v", err)
	}
}

func TestMutationKills_ExportedParseError(t *testing.T) {
	t.Parallel()
	_, err := MutationKills([]byte(`{`), 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckMutationKillsRate_Exported(t *testing.T) {
	t.Parallel()
	check := parseMutationKillsReportString(t, mutationKillsSampleFilesJSON)
	if err := CheckMutationKillsRate(check, 0); err != nil {
		t.Fatalf("CheckMutationKillsRate(0): %v", err)
	}
	if err := CheckMutationKillsRate(check, 100); err != nil {
		t.Fatalf("CheckMutationKillsRate(100): %v", err)
	}
}
