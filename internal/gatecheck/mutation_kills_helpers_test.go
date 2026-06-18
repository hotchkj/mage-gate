// Vision: Thin test wrappers around ParseMutationKillsReport for fixture-heavy cases in other *_test.go files.
package gatecheck

import (
	"bytes"
	"testing"
)

func parseMutationKillsReportString(t *testing.T, jsonData string) *MutationKillsCheck {
	t.Helper()
	check, err := ParseMutationKillsReport(bytes.NewReader([]byte(jsonData)))
	if err != nil {
		t.Fatalf("ParseMutationKillsReport() error = %v", err)
	}
	return check
}
