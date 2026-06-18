// Vision: Mutation-kills Given/When state split from scenario_state so scenario_state stays within length limits.
package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
)

var errParseMutationStatus = errors.New("parse mutation status row")

const (
	mutationStatusDataOffset = 2
)

func (s *scenarioState) setMutationKillsMinRate(percent int) error {
	s.mutationKillsMinRate = percent
	s.stepOpts["minKillRate"] = percent
	return nil
}

func (s *scenarioState) givenMutationTestResultKilledAndLived(killed, lived int) error {
	if s.mutationKillsResult == nil {
		s.mutationKillsResult = make(map[string]int)
	}
	s.mutationKillsResult["KILLED"] = killed
	s.mutationKillsResult["LIVED"] = lived
	return nil
}

func (s *scenarioState) givenMutationTestResultFromTable(table *godog.Table) error {
	parsed, err := parseMutationStatusRows(table)
	if err != nil {
		return err
	}
	s.mutationKillsResult = parsed
	return nil
}

func parseMutationStatusRows(table *godog.Table) (map[string]int, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("%w: 0: expected header row followed by at least one status row", errParseMutationStatus)
	}

	if err := validateMutationHeader(table.Rows[0]); err != nil {
		return nil, err
	}

	parsed := make(map[string]int)
	for rowNumber, row := range table.Rows[1:] {
		dataRowNumber := rowNumber + mutationStatusDataOffset
		if err := parseMutationStatusRow(row, parsed, dataRowNumber); err != nil {
			return nil, err
		}
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("%w: 2: no status rows present", errParseMutationStatus)
	}

	return parsed, nil
}

func validateMutationHeader(header *messages.PickleTableRow) error {
	if len(header.Cells) < minMutationStatusCells {
		return fmt.Errorf(
			"%w: 1: expected header row with at least %d columns (status,count), got %d: %q",
			errParseMutationStatus,
			minMutationStatusCells,
			len(header.Cells), rowValues(header),
		)
	}

	if strings.TrimSpace(header.Cells[0].Value) != "status" || strings.TrimSpace(header.Cells[1].Value) != "count" {
		return fmt.Errorf(
			"%w: 1: invalid header row: expected first two columns \"status\", \"count\"",
			errParseMutationStatus,
		)
	}
	return nil
}

func parseMutationStatusRow(row *messages.PickleTableRow, parsed map[string]int, dataRowNumber int) error {
	if len(row.Cells) < minMutationStatusCells {
		return fmt.Errorf(
			"%w: %d: expected at least %d columns (status,count), got %d: %q",
			errParseMutationStatus,
			dataRowNumber,
			minMutationStatusCells,
			len(row.Cells),
			rowValues(row),
		)
	}
	status := strings.TrimSpace(row.Cells[0].Value)
	if status == "" {
		return fmt.Errorf("%w: %d: status is required", errParseMutationStatus, dataRowNumber)
	}
	if _, exists := parsed[status]; exists {
		return fmt.Errorf(
			"%w: %d: duplicate status %q in status table",
			errParseMutationStatus,
			dataRowNumber,
			status,
		)
	}

	countStr := strings.TrimSpace(row.Cells[1].Value)
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return fmt.Errorf(
			"%w: %d: cannot parse count %q for status %q: %w",
			errParseMutationStatus,
			dataRowNumber,
			countStr,
			status,
			err,
		)
	}
	if count < 0 {
		return fmt.Errorf(
			"%w: %d: count %q for status %q must be non-negative",
			errParseMutationStatus,
			dataRowNumber,
			countStr,
			status,
		)
	}
	parsed[status] = count
	return nil
}

func rowValues(row *messages.PickleTableRow) string {
	var parts []string
	for _, cell := range row.Cells {
		parts = append(parts, cell.Value)
	}
	return strings.Join(parts, "|")
}
