// Vision: Keyed CommandRunner fake—register per-command behavior and inspect argv/sequencing without real exec.
// This package is for any consumer of cmdrunner, not gate-specific.
package cmdtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

// ErrUnhandledCommand is returned when FakeRunner receives a command with no matching response.
var ErrUnhandledCommand = errors.New("cmdtest.FakeRunner: unhandled command")

// ErrDuplicateResponseKey is returned by ValidateUniqueResponseKeys when two On() options use
// the same dispatch key without MergeDuplicateKeys.
var ErrDuplicateResponseKey = errors.New("cmdtest.FakeRunner: duplicate response key")

// ErrValidateUniqueContainsMerge means ValidateUniqueResponseKeys was called with options that
// include MergeDuplicateKeys; that helper is only for strict pre-merge option slices (merge
// defeats the uniqueness check by design).
var ErrValidateUniqueContainsMerge = errors.New(
	"cmdtest.ValidateUniqueResponseKeys: opts must not include MergeDuplicateKeys",
)

// CommandFunc is the signature for FakeRunner command response functions.
type CommandFunc func(
	ctx context.Context,
	cmd cmdrunner.Command,
	stdout, stderr io.Writer,
) error

// RunnerOption configures a FakeRunner during construction.
type RunnerOption func(*FakeRunner)

// MergeDuplicateKeys keeps the first response when duplicate On() keys are registered.
// Use when concatenating multiple RunnerOption slices so the same dispatch key may appear more
// than once (e.g. several composed handlers all register "go test").
func MergeDuplicateKeys() RunnerOption {
	return func(fr *FakeRunner) {
		fr.mergeDup = true
	}
}

// On registers a response for the given dispatch key. Keys are space-separated
// tokens matched longest-first against (name, args[0], args[1]):
//
//   - "go test"       matches name=="go" && args[0]=="test"
//   - "go tool cover" matches name=="go" && args[0]=="tool" && args[1]=="cover"
//   - "golangci-lint"  matches name=="golangci-lint" (any args)
//   - "go"            bare name key — matches any go subcommand not matched above.
//
// Keys are limited to at most three space-separated tokens (name + two args).
// Duplicate keys panic unless MergeDuplicateKeys is applied.
func On(key string, response CommandFunc) RunnerOption {
	return func(fr *FakeRunner) {
		if err := fr.registerResponse(key, response); err != nil {
			panic(fmt.Sprintf(
				"cmdtest.FakeRunner: duplicate On(%q)", key,
			))
		}
	}
}

// FakeRunner implements cmdrunner.CommandRunner with keyed dispatch and call
// recording. Not thread-safe — each test must construct its own instance.
type FakeRunner struct {
	responses map[string]CommandFunc
	calls     []cmdrunner.Command
	mergeDup  bool
}

func (fr *FakeRunner) registerResponse(key string, response CommandFunc) error {
	if _, exists := fr.responses[key]; exists {
		if fr.mergeDup {
			return nil
		}
		return fmt.Errorf("%w: duplicate On(%q)", ErrDuplicateResponseKey, key)
	}
	fr.responses[key] = response
	return nil
}

// NewFakeRunner constructs a FakeRunner with the given response options.
// Panics on duplicate On() keys unless MergeDuplicateKeys is also applied.
func NewFakeRunner(opts ...RunnerOption) *FakeRunner {
	fr := &FakeRunner{
		responses: make(map[string]CommandFunc),
	}
	for _, opt := range opts {
		opt(fr)
	}
	return fr
}

// ValidateUniqueResponseKeys verifies opts build a FakeRunner without MergeDuplicateKeys and
// without duplicate On() dispatch keys. It encodes FakeRunner registration rules only: same
// package as On/NewFakeRunner, no gate/BDD/mage imports. Callers that concatenate option slices
// (e.g. a test harness) should run this on each slice before merging if they want strict
// duplicate detection without silent first-wins behavior. opts must not include MergeDuplicateKeys.
func ValidateUniqueResponseKeys(opts ...RunnerOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrDuplicateResponseKey, r)
		}
	}()
	fr := NewFakeRunner(opts...)
	if err != nil {
		return err
	}
	if fr.mergeDup {
		return ErrValidateUniqueContainsMerge
	}
	return nil
}

// Run wraps the invocation into a Command, records it, then dispatches to the
// matching response. Unhandled commands return an error.
func (fr *FakeRunner) Run(
	ctx context.Context,
	dir string,
	stdout, stderr io.Writer,
	name string,
	args ...string,
) error {
	cmd := cmdrunner.NewCommand(dir, name, args...)
	fr.calls = append(fr.calls, cmd)

	response := fr.resolve(cmd)
	if response == nil {
		return fmt.Errorf(
			"%w: %q %v", ErrUnhandledCommand,
			name, args,
		)
	}
	return response(ctx, cmd, stdout, stderr)
}

// Calls returns a copy of all recorded invocations.
func (fr *FakeRunner) Calls() []cmdrunner.Command {
	out := make([]cmdrunner.Command, len(fr.calls))
	copy(out, fr.calls)
	return out
}

// resolve finds the longest matching response key for the given command.
// Resolution order: "name arg0 arg1" -> "name arg0" -> "name".
func (fr *FakeRunner) resolve(cmd cmdrunner.Command) CommandFunc {
	name := cmd.Name()
	arg0 := cmd.Arg(0)
	arg1 := cmd.Arg(1)

	if arg0 != "" && arg1 != "" {
		threeKey := strings.Join(
			[]string{name, arg0, arg1}, " ",
		)
		if h, ok := fr.responses[threeKey]; ok {
			return h
		}
	}
	if arg0 != "" {
		twoKey := name + " " + arg0
		if h, ok := fr.responses[twoKey]; ok {
			return h
		}
	}
	if h, ok := fr.responses[name]; ok {
		return h
	}
	return nil
}
