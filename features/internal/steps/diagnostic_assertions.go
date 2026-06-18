// Vision: Cucumber assertions on DiagnosticError text—stable ERROR/Fix/Hint snippets without brittle full dumps.
package steps

import (
	"fmt"
	"strings"
)

// assertDiagnosticBlocksOnString verifies ERROR/Fix/Hint structure in a message.
func assertDiagnosticBlocksOnString(errMsg string, expectedErr error) error {
	errorLine := lineNumberOf(errMsg, "ERROR:")
	fixLine := lineNumberOf(errMsg, "Fix:")
	hintLine := lineNumberOf(errMsg, "Hint:")
	if errorLine < 0 {
		return fmt.Errorf("%w: expected ERROR block in error output", expectedErr)
	}
	if fixLine < 0 {
		return fmt.Errorf("%w: expected Fix block in error output", expectedErr)
	}
	if hintLine < 0 {
		return fmt.Errorf("%w: expected Hint block in error output", expectedErr)
	}

	if errorLine >= fixLine || fixLine >= hintLine {
		return fmt.Errorf(
			"%w: diagnostic blocks in wrong order: ERROR line %d, Fix line %d, Hint line %d",
			expectedErr, errorLine, fixLine, hintLine,
		)
	}
	return nil
}

// lineNumberOf returns the 0-based line index of the first line whose trimmed content starts with prefix, or -1.
func lineNumberOf(text, prefix string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return i
		}
	}
	return -1
}
