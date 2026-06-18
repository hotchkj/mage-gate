// Vision: Generic pinned-tool registry for BDD scenarios—replaces per-tool state fields.
package steps

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	errStepNotInPinnedToolRegistry = errors.New("step not found in pinned tool registry")
	errInvalidLocalToolState       = errors.New("invalid local tool state")
	errInvalidToolSpec             = errors.New("invalid tool spec")
	pinnedTools                    = map[string]pinnedToolEntry{
		stepLint:             {probeName: "golangci-lint"},
		stepFormat:           {probeName: "golangci-lint"},
		stepDeadcode:         {probeName: "deadcode"},
		stepMarkdownlint:     {probeName: "gomarklint"},
		stepMutationScan:     {probeName: "gremlins"},
		stepMutationSites:    {probeName: "gremlins"},
		stepMutationCoverage: {probeName: "gremlins"},
		stepMutationKills:    {probeName: "gremlins"},
		stepCrap:             {probeName: "gocyclo"},
	}
)

// pinnedToolEntry maps a step name to its probe binary name.
type pinnedToolEntry struct {
	probeName string
}

func validatePinnedToolStep(step string) error {
	if isPinnedToolStep(step) {
		return nil
	}
	return fmt.Errorf(
		"%w: %q (expected one of: %s)",
		errStepNotInPinnedToolRegistry,
		step,
		strings.Join(pinnedToolStepNames(), ", "),
	)
}

func pinnedToolStepNames() []string {
	names := make([]string, 0, len(pinnedTools))
	for name := range pinnedTools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateLocalToolState(state string) error {
	switch state {
	case toolStateMatching, toolStateMismatched, toolStateMissing, toolStateUnprobeable:
		return nil
	default:
		return fmt.Errorf(
			"%w: %q (expected matching, mismatched, missing, or unprobeable)",
			errInvalidLocalToolState,
			state,
		)
	}
}

// setStepToolSpec stores the tool spec for a given step in stepOpts.
// Key format: "<step>ToolSpec" (e.g., "lintToolSpec", "deadcodeToolSpec").
func (s *scenarioState) setStepToolSpec(step, spec string) error {
	if err := validatePinnedToolStep(step); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return fmt.Errorf("%w: empty tool spec for step %q", errInvalidToolSpec, step)
	}
	if s.stepOpts == nil {
		s.stepOpts = make(map[string]interface{})
	}
	key := step + "ToolSpec"
	s.stepOpts[key] = trimmed
	return nil
}

// stepToolSpec retrieves the tool spec for a given step.
// Returns the spec and true if found, empty string and false otherwise.
func (s *scenarioState) stepToolSpec(step string) (string, bool) {
	key := step + "ToolSpec"
	spec, ok := s.stepOpts[key].(string)
	return spec, ok
}

// setLocalToolState stores the local tool state for a given step.
// Uses a dedicated map for local states to avoid collisions with stepOpts.
func (s *scenarioState) setLocalToolState(step, state string) error {
	if err := validatePinnedToolStep(step); err != nil {
		return err
	}
	if err := validateLocalToolState(state); err != nil {
		return err
	}
	if s.localToolStates == nil {
		s.localToolStates = make(map[string]string)
	}
	s.localToolStates[step] = state
	return nil
}

// localToolState retrieves the local tool state for a given step.
// Returns the state and true if found, empty string and false otherwise.
func (s *scenarioState) localToolState(step string) (string, bool) {
	state, ok := s.localToolStates[step]
	return state, ok
}

// pinnedToolProbeName returns the probe binary name for a given step.
// Returns an error if the step is not in the registry.
func (s *scenarioState) pinnedToolProbeName(step string) (string, error) {
	registry := pinnedTools
	entry, ok := registry[step]
	if !ok {
		return "", fmt.Errorf("%w: %q", errStepNotInPinnedToolRegistry, step)
	}
	return entry.probeName, nil
}

// expectedToolSpecForStep returns the expected tool spec for a step.
// It looks up in stepOpts using the key pattern "<step>ToolSpec".
func (s *scenarioState) expectedToolSpecForStep(step string) string {
	spec, _ := s.stepToolSpec(step)
	return spec
}

// isPinnedToolStep returns true if the step uses a pinned external tool.
func isPinnedToolStep(step string) bool {
	_, ok := pinnedTools[step]
	return ok
}

// toolStateForArgs returns the FakeRunner stub key for a pinned tool step.
// Unset local state uses go run (gatetest.ToolStateDefault); explicit states match ResolverExpectedKeys.
func (s *scenarioState) toolStateForArgs(step string) string {
	state, _ := s.localToolState(step)
	switch state {
	case toolStateMatching:
		probeName, err := s.pinnedToolProbeName(step)
		if err != nil {
			return fakeRunnerKeyGoRun
		}
		return probeName
	case toolStateMismatched, toolStateMissing:
		spec := s.expectedToolSpecForStep(step)
		if spec != "" {
			return fakeRunnerKeyGoRun + " " + spec
		}
		return fakeRunnerKeyGoRun
	default:
		return fakeRunnerKeyGoRun
	}
}
