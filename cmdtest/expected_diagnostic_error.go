// Vision: Shared ERROR/Fix/Hint golden strings for tests asserting cmdrunner.DiagnosticError layout.
package cmdtest

import (
	"fmt"
	"strings"
)

// ExpectedDiagnosticErrorString formats the canonical ERROR/Fix/Hint layout produced by
// [cmdrunner.DiagnosticError.Error].
func ExpectedDiagnosticErrorString(name, message, fix, hint, toolOutput string) string {
	var sb strings.Builder
	if name != "" {
		fmt.Fprintf(&sb, "ERROR: [%s] %s\n", name, message)
	} else {
		fmt.Fprintf(&sb, "ERROR: %s\n", message)
	}
	if fix != "" {
		fmt.Fprintf(&sb, "Fix: %s\n", fix)
	}
	if hint != "" {
		fmt.Fprintf(&sb, "Hint: %s\n", hint)
	}
	if toolOutput != "" {
		sb.WriteString(toolOutput)
	}
	return sb.String()
}
