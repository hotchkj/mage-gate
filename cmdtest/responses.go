// Vision: Portable FakeRunner responses—stdout/stderr/exit factories shared by cmdtest, gatetest, and harness tests.
package cmdtest

import (
	"context"
	"io"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

// NoopCommand always succeeds and writes nothing. Use at specific keys
// (e.g., On("go vet", NoopCommand)), never as a catch-all on bare "go".
var NoopCommand CommandFunc = func( //nolint:gochecknoglobals // test-double constant
	_ context.Context, _ cmdrunner.Command, _, _ io.Writer,
) error {
	return nil
}

// Fail returns a CommandFunc that always returns the given error.
func Fail(err error) CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
		return err
	}
}

// FailWith returns a CommandFunc that writes output to stdout before returning the error.
// Use for realistic failure fakes where the tool produces visible output before exiting.
func FailWith(err error, output string) CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
		if _, writeErr := io.WriteString(stdout, output); writeErr != nil {
			return writeErr
		}
		return err
	}
}
