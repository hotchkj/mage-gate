// Vision: Guard that mutation table fixture parsing rejects malformed and negative inputs early.
package steps

import (
	"errors"
	"strconv"
	"testing"

	messages "github.com/cucumber/messages/go/v21"
)

func TestGivenMutationTestResultFromTableAcceptsWellFormedRows(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"status", "count"},
		{"KILLED", "8"},
		{"LIVED", "2"},
	})

	if err := state.givenMutationTestResultFromTable(table); err != nil {
		t.Fatalf("givenMutationTestResultFromTable: %v", err)
	}
	if got := state.mutationKillsResult["KILLED"]; got != 8 {
		t.Fatalf("KILLED: got %d, want 8", got)
	}
	if got := state.mutationKillsResult["LIVED"]; got != 2 {
		t.Fatalf("LIVED: got %d, want 2", got)
	}
}

func TestGivenMutationTestResultFromTableRejectsMissingCountCell(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"status", "count"},
		{"KILLED", "8"},
		{"LIVED"},
	})
	err := state.givenMutationTestResultFromTable(table)
	if err == nil {
		t.Fatal("expected malformed table to fail")
	}
	if !errors.Is(err, errParseMutationStatus) {
		t.Fatalf("expected errParseMutationStatus, got %v", err)
	}
	want := `parse mutation status row: 3: expected at least 2 columns (status,count), got 1: "LIVED"`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestGivenMutationTestResultFromTableRejectsInvalidCountValue(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"status", "count"},
		{"KILLED", "not-an-int"},
	})
	err := state.givenMutationTestResultFromTable(table)
	if err == nil {
		t.Fatal("expected parse failure")
	}
	if !errors.Is(err, errParseMutationStatus) {
		t.Fatalf("expected errParseMutationStatus, got %v", err)
	}
	var numErr *strconv.NumError
	if !errors.As(err, &numErr) {
		t.Fatalf("expected strconv.NumError in chain, got %v", err)
	}
	if numErr.Num != "not-an-int" {
		t.Fatalf("NumError.Num = %q, want %q", numErr.Num, "not-an-int")
	}
}

func TestGivenMutationTestResultFromTableRejectsNegativeCount(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"status", "count"},
		{"KILLED", "-5"},
	})
	err := state.givenMutationTestResultFromTable(table)
	if err == nil {
		t.Fatal("expected negative count failure")
	}
	if !errors.Is(err, errParseMutationStatus) {
		t.Fatalf("expected errParseMutationStatus, got %v", err)
	}
	want := `parse mutation status row: 2: count "-5" for status "KILLED" must be non-negative`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestGivenMutationTestResultFromTableRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"foo", "bar"},
		{"KILLED", "8"},
	})
	err := state.givenMutationTestResultFromTable(table)
	if err == nil {
		t.Fatal("expected header validation failure")
	}
	if !errors.Is(err, errParseMutationStatus) {
		t.Fatalf("expected errParseMutationStatus, got %v", err)
	}
	want := `parse mutation status row: 1: invalid header row: expected first two columns "status", "count"`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestGivenMutationTestResultFromTableRejectsDuplicateStatuses(t *testing.T) {
	t.Parallel()

	state := newScenarioState()
	table := mutationStatusTable(t, [][]string{
		{"status", "count"},
		{"KILLED", "8"},
		{"KILLED", "2"},
	})
	err := state.givenMutationTestResultFromTable(table)
	if err == nil {
		t.Fatal("expected duplicate status validation failure")
	}
	if !errors.Is(err, errParseMutationStatus) {
		t.Fatalf("expected errParseMutationStatus, got %v", err)
	}
	want := `parse mutation status row: 3: duplicate status "KILLED" in status table`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func mutationStatusTable(t *testing.T, rows [][]string) *messages.PickleTable {
	t.Helper()

	out := make([]*messages.PickleTableRow, 0, len(rows))
	for _, row := range rows {
		cells := make([]*messages.PickleTableCell, 0, len(row))
		for _, value := range row {
			cells = append(cells, &messages.PickleTableCell{Value: value})
		}
		out = append(out, &messages.PickleTableRow{Cells: cells})
	}
	return &messages.PickleTable{Rows: out}
}
