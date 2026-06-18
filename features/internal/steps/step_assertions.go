// Vision: Register godog steps and assert resolver behavior (local vs go-run) for gate scenarios.
// Invariant: Assertions are separated from scenario state bookkeeping to satisfy repo file-size limits.
//
//nolint:revive // file-length-limit is enforced globally, this helper test file is intentionally large
package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/hotchkj/mage-gate/cmdrunner"
	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// goRunSubcommand is argv[1] when the go toolchain dispatches a pinned module via "go run".
const goRunSubcommand = "run"

var (
	errNoMutationKillsOutput        = errors.New("no mutation kills output found in step opts")
	errKillRateMismatch             = errors.New("kill rate mismatch")
	errSurvivorFileCountMismatch    = errors.New("mutations artifact survivor file count mismatch")
	errGremlinsDryRunCount          = errors.New("gremlins dry-run invocation count mismatch")
	errToolNotFound                 = errors.New("tool not found in dispatched commands")
	errToolNotLocal                 = errors.New("expected tool to run locally")
	errToolNotGoRun                 = errors.New("expected tool to run via go run")
	errToolUnavailablePassedAnyway  = errors.New("expected step to fail due to tool unavailability, but step passed")
	errToolAvailableWhenUnavailable = errors.New("expected tool to be unavailable")
	errUnexpectedDispatch           = errors.New("expected no commands to be dispatched")
)

func (s *scenarioState) assertMutationScanSucceeded() error {
	if s.runErr != nil {
		return fmt.Errorf("mutation scan: %w", s.runErr)
	}
	if !s.mutationScanOutputReady {
		return fmt.Errorf("mutation scan: %w", errMutationScanArtifactsMissing)
	}
	return nil
}

func (s *scenarioState) assertGremlinsDryRunRanOnce() error {
	if s.gremlinsDryRunInvocations != 1 {
		return fmt.Errorf("%w: want 1, got %d", errGremlinsDryRunCount, s.gremlinsDryRunInvocations)
	}
	return nil
}

func (s *scenarioState) assertMutationSitesReportDidNotInclude(path string) error {
	stepID, found := s.ensureStore().FindArtifactByStepPrefix(stepMutationScan, "mutations.json")
	if !found {
		return fmt.Errorf("%w: mutations.json from mutationscan step", errArtifactMissing)
	}
	return s.assertFilteredStoreMutationExcludesPath(stepID, path, "mutation-sites scoped report")
}

// assertToolRunsLocally checks that the current step's tool is running as a local binary.
func (s *scenarioState) assertToolRunsLocally() error {
	actual := s.actualToolCommand()
	if actual == "" {
		return errToolNotFound
	}
	if actual != commandLocalBinary {
		return fmt.Errorf("%w: %q", errToolNotLocal, actual)
	}
	return nil
}

// assertToolRunsViaGoRun checks that the current step's tool is running via go run.
func (s *scenarioState) assertToolRunsViaGoRun() error {
	actual := s.actualToolCommand()
	if actual == "" {
		return errToolNotFound
	}
	if !strings.HasPrefix(actual, fakeRunnerKeyGoRun) {
		return fmt.Errorf("%w: %q", errToolNotGoRun, actual)
	}
	return nil
}

// assertToolNotAvailable checks that the tool is not available.
// It verifies: (1) step failed, (2) no tool was found/resolved.
func (s *scenarioState) assertToolNotAvailable() error {
	if s.runErr == nil {
		return errToolUnavailablePassedAnyway
	}
	if s.actualToolCommand() != "" {
		return fmt.Errorf("%w: %q", errToolAvailableWhenUnavailable, s.actualToolCommand())
	}
	return nil
}

// actualToolCommand returns the tool's resolution strategy for the current step using step-specific calls.
// For single-step or on failure, uses recordedCalls (the failing/only step).
// For multi-step success, looks up the current step in stepCallsMap for accurate per-step assertions.
func (s *scenarioState) actualToolCommand() string {
	var calls []cmdrunner.Command

	// Try to use step-specific calls from map for multi-step scenarios.
	if len(s.stepCallsMap) > 0 {
		if stepCalls, ok := s.stepCallsMap[s.stepName]; ok && len(stepCalls) > 0 {
			calls = stepCalls[len(stepCalls)-1]
		} else {
			calls = s.recordedCalls // Fallback if step not in map
		}
	} else {
		calls = s.recordedCalls // Single-step or failure case
	}

	switch s.stepName {
	case stepLint, stepFormat, stepDeadcode, stepMarkdownlint, stepCrap,
		stepMutationScan, stepMutationSites, stepMutationKills:
		return s.getToolCommandForStep(s.stepName, calls)
	}
	return ""
}

// getToolCommandForStep inspects a specific step's calls and returns its tool resolution strategy.
// stepName is the tool step (lint, deadcode, crap, mutationsites, mutationkills).
func (s *scenarioState) getToolCommandForStep(stepName string, calls []cmdrunner.Command) string {
	probeName, err := s.pinnedToolProbeName(stepName)
	if err != nil {
		return ""
	}
	return s.findToolInCalls(calls, probeName, s.expectedToolSpecForStep(stepName))
}

// findToolInCalls searches calls for a local or go-run instance of toolName.
func (s *scenarioState) findToolInCalls(calls []cmdrunner.Command, toolName, toolSpec string) string {
	for _, call := range calls {
		if call.Name() == toolName ||
			strings.HasSuffix(call.Name(), "/"+toolName) ||
			strings.HasSuffix(call.Name(), "\\"+toolName+".exe") {
			return commandLocalBinary
		}
		if call.Name() == "go" && call.Arg(0) == goRunSubcommand {
			if toolSpec == "" {
				return fakeRunnerKeyGoRun
			}
			if call.Arg(1) == toolSpec {
				return fakeRunnerKeyGoRun + " " + call.Arg(1)
			}
		}
	}
	return ""
}

// assertStepDoesNotDispatch checks that the current step dispatched no commands.
func (s *scenarioState) assertStepDoesNotDispatch() error {
	if len(s.recordedCalls) != 0 {
		return fmt.Errorf("%w: got %d", errUnexpectedDispatch, len(s.recordedCalls))
	}
	return nil
}

// assertGremlinsDryRunDidNotRun is used for kills-only steps that must not wrap the dry-run counter.
func (s *scenarioState) assertGremlinsDryRunDidNotRun() error {
	if s.gremlinsDryRunInvocations != 0 {
		return fmt.Errorf("%w: want 0, got %d", errGremlinsDryRunCount, s.gremlinsDryRunInvocations)
	}
	return nil
}

func registerScenarioSetupSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	registerScenarioThresholdSteps(ctx, state)
	registerScenarioFixtureAndCodebaseSteps(ctx, state)
	registerScenarioToolAndMutationSetupSteps(ctx, state)
}

func registerScenarioThresholdSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	ctx.Step(`^a coverage threshold of (\d+)$`, func(n int) error {
		return state.setCoverageThreshold(n)
	})
	ctx.Step(`^a CRAP threshold of (\d+)$`, func(n int) error {
		return state.setCrapThreshold(n)
	})
	ctx.Step(`^a duration threshold of (\d+) seconds$`, func(n int) error {
		return state.setDurationThreshold(n)
	})
	ctx.Step(`^the duration threshold is ([0-9.]+) seconds$`, func(n string) error {
		var seconds float64
		if _, err := fmt.Sscanf(n, "%f", &seconds); err != nil {
			return fmt.Errorf("parse duration threshold %q: %w", n, err)
		}
		state.stepOpts["maxSeconds"] = seconds
		return nil
	})
	ctx.Step(`^a mutation threshold of (\d+) sites$`, func(n int) error {
		return state.setMutationSitesThreshold(n)
	})
	ctx.Step(`^a mutation coverage min of (\d+) percent$`, func(percent string) error {
		var pct int
		if _, err := fmt.Sscanf(percent, "%d", &pct); err != nil {
			return err
		}
		state.stepOpts["mutationCoverageMin"] = pct
		return nil
	})
	ctx.Step(`^the quality scope test file patterns include "([^"]*)"$`, state.addQualityScopeTestFilePattern)
	ctx.Step(`^the quality scope has build tag "([^"]*)"$`, state.addQualityScopeTag)
	ctx.Step(`^a lint config path of "([^"]*)"$`, state.setLintConfigPath)
	ctx.Step(`^a custom golangci-lint definition path of "([^"]*)"$`, state.setCustomGCLPath)
}

func registerScenarioFixtureAndCodebaseSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	ctx.Step(`^the fixture contains test file "([^"]*)"$`, state.givenFixtureTestFile)
	ctx.Step(`^the fixture contains source file "([^"]*)"$`, state.givenFixtureSourceFile)
	ctx.Step(
		`^the fixture contains source file outside the package inventory "([^"]*)"$`,
		state.givenFixtureUnlistedSourceFile,
	)
	ctx.Step(`^the output mode is (agent|verbose)$`, state.setOutputMode)
	ctx.Step(`^the codebase has (\d+)% test coverage$`, func(pct int) error {
		return state.givenCodebaseTestCoverage(pct)
	})
	ctx.Step(`^the codebase has no lint violations$`, state.givenLintClean)
	ctx.Step(`^the codebase has lint violations$`, state.givenLintViolations)
	ctx.Step(`^the codebase compiles cleanly$`, state.givenCompilesCleanly)
	ctx.Step(`^the codebase has vet issues$`, state.givenVetIssues)
	ctx.Step(`^the codebase fails to compile$`, state.givenFailsToCompile)
	ctx.Step(`^the codebase has failing tests$`, state.givenFailingTests)
	ctx.Step(`^the codebase has dead code$`, state.givenDeadcodeIssues)
	ctx.Step(`^the codebase has markdown violations$`, state.givenMarkdownlintIssues)
	ctx.Step(`^the codebase has excessive mutation sites$`, state.givenMutationExcessive)
	ctx.Step(`^the codebase has slow tests$`, state.givenSlowTests)
	ctx.Step(`^the codebase has fast tests with slow package wall-clock$`, state.givenFastTestsWithSlowPackageWallClock)
	ctx.Step(`^function "([^"]*)" has cyclomatic complexity (\d+)$`, func(fn string, score int) error {
		return state.givenFuncCyclomatic(fn, score)
	})
	ctx.Step(`^the module "([^"]*)" has package "([^"]*)" at "([^"]*)"$`, state.givenModulePackageAt)
	ctx.Step(
		`^package "([^"]*)" has a test event "([^"]*)" lasting ([0-9.]+) seconds$`,
		state.givenPackageTestEvent,
	)
}

func registerScenarioToolAndMutationSetupSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	ctx.Step(`^a custom lint tool spec of "([^"]*)"$`, state.setCustomLintToolSpec)
	ctx.Step(`^lint has an extra argument of "([^"]*)" specified$`, state.addLintExtraArg)
	ctx.Step(`^deadcode has an extra argument of "([^"]*)" specified$`, state.addDeadcodeExtraArg)
	ctx.Step(`^markdownlint has an extra argument of "([^"]*)" specified$`, state.addMarkdownlintExtraArg)
	ctx.Step(`^vet has an extra argument of "([^"]*)" specified$`, state.addVetExtraArg)
	ctx.Step(`^compile has an extra argument of "([^"]*)" specified$`, state.addCompileExtraArg)
	ctx.Step(`^test has an extra argument of "([^"]*)" specified$`, state.addTestExtraArg)
	ctx.Step(`^coveredtest has an extra argument of "([^"]*)" specified$`, state.addTestExtraArg)
	ctx.Step(`^mutation has an extra argument of "([^"]*)" specified$`, state.addMutationExtraArg)
	ctx.Step(`^crap has an extra argument of "([^"]*)" specified$`, state.addCrapExtraArg)
	ctx.Step(`^the package scope is "([^"]*)"$`, state.setPackageScopePattern)
	ctx.Step(`^the quality scope is "([^"]*)"$`, state.setQualityScopePattern)
	ctx.Step(`^the quality scope excludes "([^"]*)"$`, state.setQualityScopeExclude)
	ctx.Step(`^a mutation kills min rate of (\d+) percent$`, func(percent string) error {
		var pct int
		if _, err := fmt.Sscanf(percent, "%d", &pct); err != nil {
			return err
		}
		return state.setMutationKillsMinRate(pct)
	})
	ctx.Step(`^the mutation test result has (\d+) killed and (\d+) lived mutations$`, func(killed, lived int) error {
		return state.givenMutationTestResultKilledAndLived(killed, lived)
	})
	ctx.Step(`^the mutation test result has mixed statuses:$`, func(table *godog.Table) error {
		return state.givenMutationTestResultFromTable(table)
	})
}

func registerScenarioRunSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	const (
		stepMutationCoverageFromKills = `^mutation coverage is evaluated from full-run artifacts$`
		stepMutationSitesFromKills    = `^mutation sites are evaluated from full-run artifacts$`
	)
	// Single pattern: comma-separated step names (one or more). Variations live in the list, not in new step wording.
	ctx.Step(`^the gate runs steps (.+)$`, state.runGateStepsFromList)
	ctx.Step(stepMutationCoverageFromKills, state.runEvaluateMutationCoverageFromFullRunArtifacts)
	ctx.Step(stepMutationSitesFromKills, state.runEvaluateMutationSitesFromFullRunArtifacts)
}

func registerScenarioAssertionSteps(ctx *godog.ScenarioContext, state *scenarioState) {
	const (
		stepMutationCoverageEvalNotInclude = `^the mutation-coverage evaluation did not include "([^"]*)"$`
	)
	ctx.Step(`^the step passes$`, state.assertStepPasses)
	ctx.Step(`^the step fails$`, state.assertStepFails)
	ctx.Step(`^the configuration is rejected$`, state.assertConfigurationRejected)
	ctx.Step(`^the output is empty$`, state.assertOutputEmpty)
	ctx.Step(`^the output is empty except progress lines$`, state.assertOutputEmptyAfterRemovingProgress)
	ctx.Step(`^the output is an ERROR/Fix/Hint diagnostic$`, state.assertOutputIsErrorFixHintDiagnostic)
	ctx.Step(`^the output contains the tool's full output$`, state.assertOutputContainsToolFullOutput)
	ctx.Step(`^the artifact store contains "([^"]*)"$`, state.assertArtifactStoreContains)
	ctx.Step(`^the artifact store contains "([^"]*)" from the "([^"]*)" step$`, state.assertArtifactFromStep)
	ctx.Step(`^the artifact provenance records tool "([^"]*)"$`, state.assertProvenanceTool)
	ctx.Step(`^the artifact provenance records the configured scope$`, state.assertProvenanceScope)
	ctx.Step(`^the "([^"]*)" step does not execute$`, state.assertStepDidNotExecute)
	ctx.Step(`^the following steps do not run: "([^"]*)", "([^"]*)"$`, state.assertFollowingStepsDoNotRun)
	// Generic pinned-tool steps (replaces per-tool local state steps)
	ctx.Step(`^the tool spec for "([^"]*)" is "([^"]*)"$`, state.setStepToolSpec)
	ctx.Step(`^the local tool for "([^"]*)" is "([^"]*)"$`, state.setLocalToolState)

	ctx.Step(`^the tool runs locally$`, state.assertToolRunsLocally)
	ctx.Step(`^the tool runs via go run$`, state.assertToolRunsViaGoRun)
	ctx.Step(`^the tool is not available$`, state.assertToolNotAvailable)

	// Unified no-dispatch (replaces all "no X command was recorded" steps)
	ctx.Step(`^the step does not dispatch any commands$`, state.assertStepDoesNotDispatch)

	ctx.Step("^the command `([^`]+)` is run with arguments:$", state.assertCommandRunWithArguments)
	ctx.Step(`^the gremlins dry-run command did not run$`, state.assertGremlinsDryRunDidNotRun)
	ctx.Step(`^the kill rate is (\d+) percent$`, func(percent string) error {
		var pct int
		if _, err := fmt.Sscanf(percent, "%d", &pct); err != nil {
			return err
		}
		return state.assertKillRateIs(pct)
	})
	ctx.Step(`^the mutations artifact has (\d+) survivor files?$`, state.assertMutationsArtifactSurvivorFileCount)
	ctx.Step(`^the mutations artifact has no survivor files$`, state.assertMutationsArtifactNoSurvivorFiles)
	ctx.Step(`^the error is ([a-zA-Z0-9_]+)$`, func(errName string) error {
		return state.assertErrorIs(errName)
	})
	ctx.Step(`^mutation scan succeeds$`, state.assertMutationScanSucceeded)
	ctx.Step(`^the gremlins dry-run command ran once$`, state.assertGremlinsDryRunRanOnce)
	ctx.Step(`^the mutation-sites report did not include "([^"]*)"$`, state.assertMutationSitesReportDidNotInclude)
	ctx.Step(stepMutationCoverageEvalNotInclude, state.assertMutationCoverageEvaluationDidNotInclude)
	ctx.Step(`^the mutation-kills evaluation did not include "([^"]*)"$`, state.assertMutationKillsEvaluationDidNotInclude)
	ctx.Step(`^the mutation-sites evaluation did not include "([^"]*)"$`, state.assertMutationKillsEvaluationDidNotInclude)
}

var stepFailurePrefixes = []string{ //nolint:gochecknoglobals // test constant
	"lint failed:",
	"deadcode failed:",
	"markdownlint failed:",
	"compile failed:",
	"vet failed:",
	"test failed:",
	"coverage failed:",
	"crap failed:",
	"duration failed:",
	"qualityscopeinventory failed:",
	"mutationscan failed:",
	"mutationsites failed:",
	"mutationcoverage failed:",
	"mutationkills failed:",
}

func hasStepFailurePrefix(errStr string) bool {
	for _, prefix := range stepFailurePrefixes {
		if strings.HasPrefix(errStr, prefix) {
			return true
		}
	}
	return false
}

// requireRecognizedVerboseToolOutput checks that verbose display forwarded real tool stdout
// and that every non-empty line is recognizable. Markers align with gatetest
// fake responses used in BDD.
func (s *scenarioState) requireRecognizedVerboseToolOutput(out string) error {
	filtered := removeProgressLines(out)
	if strings.TrimSpace(filtered) == "" {
		return fmt.Errorf("%w: success but output buffer is empty", errVerboseOutputExpected)
	}
	return s.requireAllLinesRecognizableVerbose(out, s.expectedProgressTitlesForCurrentOutput())
}

// requireAllLinesRecognizableVerbose fails on the first non-empty line that does not match a known fake-output prefix.
func (s *scenarioState) requireAllLinesRecognizableVerbose(
	out string,
	allowedProgressTitles map[string]struct{},
) error {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !s.recognizableVerboseToolOutputLine(line, allowedProgressTitles) {
			return fmt.Errorf("%w: unrecognized line in verbose tool output: %q", errVerboseOutputExpected, line)
		}
	}
	return nil
}

// knownVerboseToolOutputPrefixes lists line prefixes that identify recognized tool output
// from gatetest fake responses in BDD scenarios. Split from the function body
// to keep recognizableVerboseToolOutputLine below the cyclomatic complexity limit.
var knownVerboseToolOutputPrefixes = []string{ //nolint:gochecknoglobals // test constant
	"golangci-lint:",
	"go vet:",
	"go build:",
	"deadcode:",
	"gomarklint:",
	`{"Action":`,
	"total:\t(statements)\t",
	"file.go:10:\t",
	"example.com/mod",
	`{"Path":`,
	"# ",
}

func (s *scenarioState) recognizableVerboseToolOutputLine(line string, allowedProgressTitles map[string]struct{}) bool {
	if title, ok := parseProgressLineTitle(line); ok {
		if len(allowedProgressTitles) == 0 {
			return isStepStartLine(line)
		}
		_, ok = allowedProgressTitles[title]
		return ok
	}
	for _, prefix := range knownVerboseToolOutputPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return isGoListDirectoryOutput(line) ||
		isSourceDiagnosticLine(line) ||
		isMarkdownDiagnosticLine(line) ||
		isSyntheticGocycloLine(line)
}

func isProgressStartLine(line string) bool {
	return isStepStartLine(line)
}

var verboseProgressTitlesByStep = map[string]map[string]struct{}{ //nolint:gochecknoglobals // test constant
	stepLint: {
		"Lint": {},
	},
	stepFormat: {
		"Format": {},
	},
	stepDeadcode: {
		"Deadcode": {},
	},
	stepMarkdownlint: {
		"Markdown lint": {},
	},
	stepVet: {
		"Vet": {},
	},
	stepCompile: {
		"Compile": {},
	},
	stepTest: {
		"Test": {},
	},
	stepCoveredTest: {
		"Covered Test": {},
	},
	stepCoverage: {
		"Coverage": {},
	},
	stepCrap: {
		"CRAP": {},
	},
	stepDuration: {
		"Duration": {},
	},
	stepQualityInventory: {
		"Quality Scope Inventory": {},
	},
	stepMutationScan: {
		"Mutation Scan": {},
	},
	stepMutationSites: {
		"Mutation Sites": {},
		"Mutation Scan":  {},
	},
	stepMutationCoverage: {
		"Mutation Coverage": {},
	},
	stepMutationKills: {
		"Mutation Scan":  {},
		"Mutation Kills": {},
	},
}

func (s *scenarioState) expectedProgressTitlesForCurrentOutput() map[string]struct{} {
	allowed := make(map[string]struct{})
	addVerboseProgressTitles(allowed, s.stepsRan...)
	addVerboseProgressTitles(allowed, s.stepName)
	// Keep existing mutation scan helper behavior where output evidence can be included
	// only for the consumer step explicitly tied to scan-output assertions.
	if s.mutationScanVerboseEvidenceForCurrentStep() != "" && s.stepName == stepMutationSites {
		addVerboseProgressTitles(allowed, stepMutationScan)
	}
	return allowed
}

func addVerboseProgressTitles(dst map[string]struct{}, steps ...string) {
	for _, stepName := range steps {
		for title := range stepAllowedProgressTitles(stepName) {
			dst[title] = struct{}{}
		}
	}
}

func stepAllowedProgressTitles(stepName string) map[string]struct{} {
	titles, ok := verboseProgressTitlesByStep[stepName]
	if !ok {
		return nil
	}
	return titles
}

func parseProgressLineTitle(line string) (string, bool) {
	if !strings.HasSuffix(line, "...") {
		return "", false
	}
	withoutSuffix := strings.TrimSuffix(line, "...")
	if withoutSuffix == "" {
		return "", false
	}
	if strings.HasSuffix(withoutSuffix, "]") {
		qualStart := strings.LastIndex(withoutSuffix, " [")
		if qualStart <= 0 {
			return "", false
		}
		withoutSuffix = withoutSuffix[:qualStart]
	}
	if !isStepStartLine(line) {
		return "", false
	}
	return withoutSuffix, true
}

var stepStartLineTitles = map[string]struct{}{
	"Lint":                    {},
	"Format":                  {},
	"Deadcode":                {},
	"Markdown lint":           {},
	"Vet":                     {},
	"Compile":                 {},
	"Test":                    {},
	"Covered Test":            {},
	"Coverage":                {},
	"CRAP":                    {},
	"Duration":                {},
	"Quality Scope Inventory": {},
	"Mutation Scan":           {},
	"Mutation Sites":          {},
	"Mutation Coverage":       {},
	"Mutation Kills":          {},
}

func isStepStartLine(line string) bool {
	if !strings.HasSuffix(line, "...") {
		return false
	}
	withoutSuffix := strings.TrimSuffix(line, "...")
	if withoutSuffix == "" {
		return false
	}
	if strings.HasSuffix(withoutSuffix, "]") {
		qualStart := strings.LastIndex(withoutSuffix, " [")
		if qualStart <= 0 {
			return false
		}
		withoutSuffix = withoutSuffix[:qualStart]
	}
	_, ok := stepStartLineTitles[withoutSuffix]
	return ok
}

func removeProgressLines(out string) string {
	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isProgressStartLine(trimmed) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// sourceDiagnosticMinParts is the minimum number of colon-separated parts in
// a source diagnostic line (file:line:col:message).
const sourceDiagnosticMinParts = 4

// isSourceDiagnosticLine matches lines like "pkg/foo.go:10:5: message" from
// compiler errors, vet diagnostics, and lint findings.
func isSourceDiagnosticLine(line string) bool {
	parts := strings.SplitN(line, ":", sourceDiagnosticMinParts)
	return len(parts) >= sourceDiagnosticMinParts && strings.HasSuffix(parts[0], ".go")
}

// isMarkdownDiagnosticLine matches gomarklint output like "README.md:1:1: MD041/...".
func isMarkdownDiagnosticLine(line string) bool {
	parts := strings.SplitN(line, ":", sourceDiagnosticMinParts)
	return len(parts) >= sourceDiagnosticMinParts && strings.HasSuffix(parts[0], ".md")
}

// isGoListDirectoryOutput matches go list -f {{.Dir}} output from BDD tests.
// These are directory paths rooted at the synthetic module directory "/mod".
func isGoListDirectoryOutput(line string) bool {
	cleaned := fsnorm.Canonical(line)
	return strings.HasPrefix(cleaned, "/mod")
}

// isSyntheticGocycloLine matches gatetest.Gocyclo output: "<n> pkg <func> file.go:1:1".
func isSyntheticGocycloLine(line string) bool {
	parts := strings.Fields(line)
	if len(parts) < gocycloLineMinFields {
		return false
	}
	if parts[1] != "pkg" {
		return false
	}
	last := parts[len(parts)-1]
	return strings.HasSuffix(last, "file.go:1:1")
}

func (s *scenarioState) assertKillRateIs(percent int) error {
	mutKillsOut, ok := s.stepOpts["mutationKillsOutput"].(qg.MutationKillsOutput)
	if !ok {
		return fmt.Errorf("%w", errNoMutationKillsOutput)
	}
	actual, err := mutKillsOut.KillRatePercent()
	if err != nil {
		return err
	}
	if actual != float64(percent) {
		return fmt.Errorf("%w: got %.2f%%, want %d%%", errKillRateMismatch, actual, percent)
	}
	return nil
}

func (s *scenarioState) assertMutationsArtifactSurvivorFileCount(want int) error {
	store := s.ensureStore()
	stepID, found := store.FindArtifactByStepPrefix(stepMutationKills, "mutations.json")
	if !found {
		return fmt.Errorf("%w: mutations.json from mutationkills step", errArtifactMissing)
	}
	data, err := store.Read(stepID, "mutations.json")
	if err != nil {
		return err
	}
	result, err := gatecheck.MutationKills(data, 0)
	if err != nil {
		return err
	}
	survived := 0
	for _, f := range result.Check.Files {
		if f.Lived > 0 {
			survived++
		}
	}
	if survived != want {
		return fmt.Errorf("%w: expected %d, got %d", errSurvivorFileCountMismatch, want, survived)
	}
	return nil
}

func (s *scenarioState) assertMutationsArtifactNoSurvivorFiles() error {
	return s.assertMutationsArtifactSurvivorFileCount(0)
}

// InitializeGateScenario registers parameterized step BDD steps.
func InitializeGateScenario(ctx *godog.ScenarioContext) {
	state := newScenarioState()
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		state.reset()
		return ctx, nil
	})

	registerScenarioSetupSteps(ctx, state)
	registerScenarioRunSteps(ctx, state)
	registerScenarioAssertionSteps(ctx, state)
}
