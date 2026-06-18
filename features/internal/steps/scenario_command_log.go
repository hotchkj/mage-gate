// Vision: Canonical command argv assertions for BDD scenarios.
package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

var (
	errCommandArgumentsMissing = errors.New("command arguments doc string missing")
	errCommandRunNotFound      = errors.New("command run not found")
	errCommandNoneHasArguments = errors.New("none command expectation cannot have arguments")
	errUnexpectedToolCommand   = errors.New("expected no resolved tool command")
)

type commandRun struct {
	name string
	args []string
}

const packageInventoryFormat = gatecheck.QualityScopeListFormat

func (s *scenarioState) commandLog() []commandRun {
	calls := s.commandLogCalls()
	out := make([]commandRun, 0, len(calls))
	for _, call := range calls {
		out = append(out, commandRun{
			name: call.Name(),
			args: normalizedCommandArgs(call),
		})
	}
	return out
}

func (s *scenarioState) commandLogCalls() []cmdrunner.Command {
	// On failures and single-step runs, keep the active step view for precision.
	if s.runErr != nil {
		return s.allDispatchedCalls
	}
	if len(s.stepsRan) <= 1 {
		return s.commandLogSingleStep()
	}

	if calls := s.commandLogForStepFlow(); len(calls) > 0 {
		return calls
	}

	return s.allDispatchedCalls
}

func (s *scenarioState) commandLogSingleStep() []cmdrunner.Command {
	if callsForStep, ok := s.stepCallsMap[s.stepName]; ok && len(callsForStep) > 0 {
		return lastNonEmptyCommandGroup(callsForStep)
	}
	if len(s.recordedCalls) > 0 {
		return s.recordedCalls
	}
	return s.allDispatchedCalls
}

func lastNonEmptyCommandGroup(groups [][]cmdrunner.Command) []cmdrunner.Command {
	for i := len(groups) - 1; i >= 0; i-- {
		if len(groups[i]) > 0 {
			return groups[i]
		}
	}
	return nil
}

func (s *scenarioState) commandLogForStepFlow() []cmdrunner.Command {
	// For successful multi-step flows, return commands in step execution order.
	calls := make([]cmdrunner.Command, 0)
	stepIndices := make(map[string]int, len(s.stepsRan))
	for _, step := range s.stepsRan {
		stepCalls, ok := s.stepCallsMap[step]
		if !ok || len(stepCalls) == 0 {
			continue
		}
		idx := stepIndices[step]
		if idx >= len(stepCalls) {
			continue
		}
		stepIndices[step] = idx + 1
		calls = append(calls, stepCalls[idx]...)
	}
	return calls
}

func normalizedCommandArgs(call cmdrunner.Command) []string {
	args := call.Args()
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = normalizeCommandArg(arg)
	}
	return out
}

func normalizedExpectedCommandArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, normalizeCommandArg(arg))
	}
	return out
}

func normalizeCommandArg(arg string) string {
	if arg == packageInventoryFormat {
		return "<package-inventory-format>"
	}
	if !isPathLikeCommandArgument(arg) {
		return arg
	}
	slashed := filepath.ToSlash(arg)
	for _, name := range []string{
		"coverage-filtered.out",
		"coverage.out",
		"cover-func.txt",
		"gocyclo.txt",
		"mutations.json",
		"test-events.jsonl",
	} {
		token := "<artifact>/" + name
		if strings.HasSuffix(slashed, "/"+name) {
			if idx := strings.LastIndex(slashed, "="); idx >= 0 {
				return slashed[:idx+1] + token
			}
			return token
		}
	}
	return slashed
}

//nolint:gochecknoglobals // helper set for canonical arg detection
var pathLikeCommandArgArtifacts = map[string]struct{}{
	"coverage-filtered.out": {},
	"coverage.out":          {},
	"cover-func.txt":        {},
	"gocyclo.txt":           {},
	"mutations.json":        {},
	"test-events.jsonl":     {},
}

func isPathLikeCommandArgument(arg string) bool {
	if strings.HasPrefix(arg, ".") || strings.ContainsAny(arg, "/\\") {
		return true
	}
	if len(arg) > 1 && arg[1] == ':' {
		return true
	}
	if eq := strings.LastIndex(arg, "="); eq >= 0 {
		_, ok := pathLikeCommandArgArtifacts[arg[eq+1:]]
		return ok
	}
	return false
}

func docStringArguments(doc *godog.DocString) ([]string, error) {
	if doc == nil {
		return nil, errCommandArgumentsMissing
	}
	normalized := strings.ReplaceAll(doc.Content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	args := make([]string, 0, len(lines))
	for _, line := range lines {
		arg := strings.TrimSpace(line)
		if arg == "" {
			continue
		}
		args = append(args, arg)
	}
	if len(args) == 1 {
		return strings.Fields(args[0]), nil
	}
	return args, nil
}

func (s *scenarioState) assertCommandRunWithArguments(command string, doc *godog.DocString) error {
	want, err := docStringArguments(doc)
	if err != nil {
		return err
	}
	want = normalizedExpectedCommandArgs(want)
	if command == "none" {
		if len(want) > 0 {
			return fmt.Errorf("%w: %v", errCommandNoneHasArguments, want)
		}
		return s.assertNoResolvedToolCommand()
	}
	return s.assertCommandRun(command, want)
}

func (s *scenarioState) assertNoResolvedToolCommand() error {
	actual := s.actualToolCommand()
	if actual == "" {
		return nil
	}
	return fmt.Errorf("%w: %q", errUnexpectedToolCommand, actual)
}

func (s *scenarioState) assertCommandRun(command string, want []string) error {
	for _, run := range s.commandLog() {
		if run.name == command && slices.Equal(run.args, want) {
			return nil
		}
	}
	return fmt.Errorf("%w: command=%q args=%v calls=%s",
		errCommandRunNotFound, command, want, formatCommandLog(s.commandLog()))
}

func formatCommandLog(log []commandRun) string {
	parts := make([]string, 0, len(log))
	for _, run := range log {
		parts = append(parts, fmt.Sprintf("%s %q", run.name, run.args))
	}
	return strings.Join(parts, "; ")
}
