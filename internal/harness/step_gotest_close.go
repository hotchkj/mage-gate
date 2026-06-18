// Vision: Combine primary go test errors with deferred profile Close failures so deferred I/O is never silent.
package harness

import (
	"errors"
	"fmt"
)

// joinStepTestCloseErr merges a primary step error with a deferred Close error.
func joinStepTestCloseErr(primary, closeErr error) error {
	if closeErr == nil {
		return primary
	}
	closeWrapped := fmt.Errorf("%w: close test events file: %w", ErrTestFailed, closeErr)
	if primary == nil {
		return closeWrapped
	}
	return errors.Join(primary, closeWrapped)
}
