// Vision: Shared validator for `./...`-style package patterns in scopes and options (reject flags, empties).
package gate

import (
	"fmt"
	"strings"
)

func validateGoTestPackagePattern(pattern string, ifEmpty error) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", ifEmpty
	}
	if strings.HasPrefix(pattern, "-") {
		return "", fmt.Errorf("%w: package pattern must not start with '-', got %q", ErrInvalidOption, pattern)
	}
	return pattern, nil
}
