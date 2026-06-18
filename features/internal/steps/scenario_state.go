// Vision: Central godog scenario state—runner, modes, roots—and step defs wired from smaller split files.
package steps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

var (
	errExpectedErrorButGotNil            = errors.New("expected error but got nil")
	errUnknownErrorType                  = errors.New("unknown error type")
	errParseTestEventElapsed             = errors.New("parse test event elapsed")
	errPackageTestEventAlreadyRegistered = errors.New("package test event already registered")

	errModuleSourceNotDerivable     = errors.New("module source root not derivable from scenario package dirs")
	errNoQualityScopePackageMatches = errors.New("no packages match quality scope in scenario module map")
	errFixtureTestRegisterPackages  = errors.New("register module packages first (fixture test file step)")
	errFixtureTestPackageUnmapped   = errors.New("fixture test file does not map to a registered package")
)

const minMutationStatusCells = 2

// scenarioState holds per-scenario configuration and results for BDD steps.
// Invariants:
//   - Lint: exactly one of lintClean/lintDirty is set per lint scenario.
//   - Compile: compileClean and compileFails are mutually exclusive (compile step = go build).
//   - Coverage/CRAP/Duration/Mutation: thresholds stored in stepOpts are set by
//     Background Given steps; codebase state (testCovPercent, testCovExplicit, gocycloScores) by Scenario Givens.
//   - After runGateSteps: runErr holds the step result; Then steps assert against it.
type scenarioState struct {
	responses  []cmdtest.RunnerOption
	stepOpts   map[string]interface{}
	outputMode qg.OutputMode
	output     *bytes.Buffer
	runErr     error
	stepName   string

	mem           qg.FileOps
	store         *qg.ArtifactStore
	gocycloScores map[string]int
	modulePath    string
	pkgImport     string
	pkgDir        string
	// modulePackages accumulates import path -> package list info for go list fakes (multi-package scenarios).
	modulePackages map[string]gatetest.PackageListInfo
	testCovPercent int
	// testCovExplicit is true after "the codebase has N% test coverage"; when false, fakes use the
	// default successful profile (see goTestOpts).
	testCovExplicit    bool
	lintClean          bool
	lintDirty          bool
	compileClean       bool
	vetIssues          bool
	compileFails       bool
	testFails          bool
	deadcodeIssues     bool
	markdownlintIssues bool
	mutationExcessive  bool
	slowTests          bool
	fastTestsSlowWall  bool
	packageTestEvents  map[string]packageTestEventSpec

	stepsRan                  []string
	lastMutationScanOut       qg.MutationScanOutput
	mutationScanOutputReady   bool
	gremlinsDryRunInvocations int
	lastArtifact              string
	lastArtifactID            string
	recordedCalls             []cmdrunner.Command
	allDispatchedCalls        []cmdrunner.Command              // all calls for all steps in a When
	stepCallsMap              map[string][][]cmdrunner.Command // calls keyed by step name, then step occurrence
	// localToolStates: step -> matching | mismatched | missing | unprobeable.
	localToolStates map[string]string

	lintExtraArgs         []string
	deadcodeExtraArgs     []string
	markdownlintExtraArgs []string
	vetExtraArgs          []string
	compileExtraArgs      []string
	testExtraArgs         []string
	mutationExtraArgs     []string
	crapExtraArgs         []string

	mutationKillsMinRate int
	mutationKillsResult  map[string]int  // counts by status
	qualityScope         qg.QualityScope // populated during runGateSteps for downstream steps

	pkgScopePattern              string // overrides "./..." when set by Given
	qualityScopePattern          string // overrides "./..." when set by Given
	qualityScopeTags             []string
	qualityScopeExcludes         []string // [Exclude] segments (append only; each Given adds one)
	qualityScopeTestFilePatterns []string

	// mutationScanVerboseEvidence preserves display text from the gremlins dry-run.
	mutationScanVerboseEvidence string
	// mutationScanVerboseEvidenceForStep scopes the cached mutation-scan output
	// to the step that should consume it.
	mutationScanVerboseEvidenceForStep string
}

type packageTestEventSpec struct {
	testName string
	elapsed  float64
}

const (
	crapCoverFile        = "file.go"
	crapCoverLine        = 10
	testPkgName          = "example.com/mod/pkg"
	coverFilePerm        = 0o600
	coverDirPerm         = 0o700
	gocycloLineMinFields = 4
	stepLint             = "lint"
	stepFormat           = "format"
	stepCompile          = "compile"
	stepDeadcode         = "deadcode"
	stepMarkdownlint     = "markdownlint"
	stepVet              = "vet"
	stepTest             = "test"
	stepQualityInventory = "qualityscopeinventory"
	stepCoveredTest      = "coveredtest"
	stepCoverage         = "coverage"
	stepCrap             = "crap"
	stepDuration         = "duration"
	stepMutationScan     = "mutationscan"
	stepMutationSites    = "mutationsites"
	stepMutationCoverage = "mutationcoverage"
	stepMutationKills    = "mutationkills"
	toolStateMatching    = "matching"
	toolStateMismatched  = "mismatched"
	toolStateMissing     = "missing"
	toolStateUnprobeable = "unprobeable"
	commandLocalBinary   = "local binary"
	// fakeRunnerKeyGoRun is the FakeRunner stub key for go-run tool resolution (no module spec).
	fakeRunnerKeyGoRun = "go run"
)

var (
	errLintHarness         = errors.New("golangci-lint: issues found")
	errVetHarness          = errors.New("go vet: failure")
	errCompileHarness      = errors.New("go build: failure")
	errMarkdownlintHarness = errors.New("gomarklint: violations found")
	errTestHarness         = errors.New("go test: failure")

	errStepFail              = errors.New("expected step failure")
	errStepPass              = errors.New("expected step success")
	errConfigReject          = errors.New("expected configuration rejection")
	errOutputEmpty           = errors.New("expected output buffer empty")
	errOutputNotDiag         = errors.New("expected ERROR/Fix/Hint diagnostic")
	errVerboseOutputExpected = errors.New("verbose display: expected visible tool output")
	errContractFail          = errors.New("step command contract not satisfied")
	errUnsupportedStep       = errors.New("unsupported step")
	errUnknownOutputMode     = errors.New("unknown output mode")
	errArtifactMissing       = errors.New("artifact not found in store")
	errProvenanceMissing     = errors.New("provenance missing")
	errStepDidExecute        = errors.New("step executed but should not have")

	errGateStepListEmpty            = errors.New("gate step list is empty")
	errGateStepListBadSegment       = errors.New("empty segment in gate step list")
	errNoGateSteps                  = errors.New("no gate steps")
	errScopesRequired               = errors.New("scenario must declare both package scope and quality scope in Gherkin")
	errMutationScanArtifactsMissing = errors.New(
		"mutation scan artifacts missing: run steps qualityscopeinventory, mutationscan first",
	)
)

var lintCleanResponse cmdtest.CommandFunc = func( //nolint:gochecknoglobals // BDD response constant
	_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer,
) error {
	_, err := io.WriteString(stdout, "golangci-lint: 0 issues\n")
	return err
}

var vetCleanResponse cmdtest.CommandFunc = func( //nolint:gochecknoglobals // BDD response constant
	_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer,
) error {
	_, err := io.WriteString(stdout, "go vet: packages checked\n")
	return err
}

var compileCleanResponse cmdtest.CommandFunc = func( //nolint:gochecknoglobals // BDD response constant
	_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer,
) error {
	_, err := io.WriteString(stdout, "go build: success\n")
	return err
}

// deadcodeCleanResponse writes its status to stderr so that stdout remains empty.
// StepDeadcode checks strings.TrimSpace(result.Stdout) to detect findings;
// writing to stderr keeps the clean signal correct while still being visible
// through the DisplayRunner on CI.
var deadcodeCleanResponse cmdtest.CommandFunc = func( //nolint:gochecknoglobals // BDD response constant
	_ context.Context, _ cmdrunner.Command, _, stderr io.Writer,
) error {
	_, err := io.WriteString(stderr, "deadcode: no issues\n")
	return err
}

// markdownlintCleanResponse writes status to stderr so stdout stays empty on success.
var markdownlintCleanResponse cmdtest.CommandFunc = func( //nolint:gochecknoglobals // BDD response constant
	_ context.Context, _ cmdrunner.Command, _, stderr io.Writer,
) error {
	_, err := io.WriteString(stderr, "gomarklint: no issues\n")
	return err
}

func lintDirtyResponse() cmdtest.CommandFunc {
	return gatetest.FailWith(errLintHarness, "pkg/foo.go:10:5: unused variable 'x' (deadcode)\n")
}

func vetDirtyResponse() cmdtest.CommandFunc {
	return gatetest.FailWith(errVetHarness, "# example.com/mod/pkg\npkg/foo.go:15:2: unreachable code\n")
}

func compileDirtyResponse() cmdtest.CommandFunc {
	return gatetest.FailWith(errCompileHarness, "# example.com/mod/pkg\npkg/foo.go:5:3: undefined: bar\n")
}

func deadcodeDirtyResponse() cmdtest.CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
		_, err := io.WriteString(stdout, "example.com/mod/pkg.UnusedFunc\n")
		return err
	}
}

const markdownViolationDiagnostic = "README.md:1:1: MD041/first-line-heading/first-line-h1: " +
	"First line in a file should be a top-level heading\n"

func markdownlintDirtyResponse() cmdtest.CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, _, stderr io.Writer) error {
		if _, err := io.WriteString(stderr, markdownViolationDiagnostic); err != nil {
			return err
		}
		return errMarkdownlintHarness
	}
}

func testDirtyResponse() cmdtest.CommandFunc {
	return gatetest.FailWith(errTestHarness,
		`{"Action":"run","Package":"example.com/mod/pkg","Test":"TestFail"}`+"\n"+
			`{"Action":"fail","Package":"example.com/mod/pkg","Test":"TestFail","Elapsed":0.01}`+"\n"+
			`{"Action":"fail","Package":"example.com/mod/pkg","Elapsed":0.01}`+"\n")
}

func newScenarioState() *scenarioState {
	return &scenarioState{
		stepOpts:      make(map[string]interface{}),
		outputMode:    qg.OutputModeAgent,
		output:        &bytes.Buffer{},
		gocycloScores: make(map[string]int),
		stepCallsMap:  make(map[string][][]cmdrunner.Command),
	}
}

func (s *scenarioState) reset() {
	s.resetOutputAndCommandState()
	s.resetContextState()
	s.resetStepState()
	s.resetFailureState()
	s.resetMutationState()
	s.resetCoverageFake()
	s.mutationScanVerboseEvidence = ""
	s.mutationScanVerboseEvidenceForStep = ""
}

func (s *scenarioState) resetCoverageFake() {
	s.testCovPercent = 0
	s.testCovExplicit = false
}

func (s *scenarioState) setCoverageThreshold(n int) error {
	s.stepOpts["minPercent"] = float64(n)
	return nil
}

func (s *scenarioState) setCrapThreshold(n int) error {
	s.stepOpts["maxScore"] = float64(n)
	return nil
}

func (s *scenarioState) setDurationThreshold(n int) error {
	s.stepOpts["maxSeconds"] = float64(n)
	return nil
}

func (s *scenarioState) setMutationSitesThreshold(n int) error {
	s.stepOpts["maxSites"] = n
	return nil
}

func (s *scenarioState) setLintConfigPath(path string) error {
	s.stepOpts["lintConfig"] = path
	return nil
}

func (s *scenarioState) setCustomGCLPath(path string) error {
	s.stepOpts["customGCLPath"] = path
	return nil
}

func (s *scenarioState) setCustomLintToolSpec(spec string) error {
	s.stepOpts["customLintSpec"] = spec
	return nil
}

func (s *scenarioState) setOutputMode(mode string) error {
	switch strings.ToLower(mode) {
	case "agent":
		s.outputMode = qg.OutputModeAgent
	case "verbose":
		s.outputMode = qg.OutputModeVerbose
	default:
		return fmt.Errorf("%w: %q", errUnknownOutputMode, mode)
	}
	return nil
}

func (s *scenarioState) givenCodebaseTestCoverage(pct int) error {
	s.testCovPercent = pct
	s.testCovExplicit = true
	return nil
}

func (s *scenarioState) givenLintClean() error          { s.lintClean = true; return nil }
func (s *scenarioState) givenLintViolations() error     { s.lintDirty = true; return nil }
func (s *scenarioState) givenCompilesCleanly() error    { s.compileClean = true; return nil }
func (s *scenarioState) givenVetIssues() error          { s.vetIssues = true; return nil }
func (s *scenarioState) givenFailsToCompile() error     { s.compileFails = true; return nil }
func (s *scenarioState) givenFailingTests() error       { s.testFails = true; return nil }
func (s *scenarioState) givenDeadcodeIssues() error     { s.deadcodeIssues = true; return nil }
func (s *scenarioState) givenMarkdownlintIssues() error { s.markdownlintIssues = true; return nil }
func (s *scenarioState) givenMutationExcessive() error {
	s.mutationExcessive = true
	return nil
}
func (s *scenarioState) givenSlowTests() error { s.slowTests = true; return nil }

func (s *scenarioState) givenFastTestsWithSlowPackageWallClock() error {
	s.fastTestsSlowWall = true
	return nil
}

func (s *scenarioState) givenFuncCyclomatic(fn string, score int) error {
	s.gocycloScores[fn] = score
	return nil
}

func (s *scenarioState) givenPackageTestEvent(importPath, testName, elapsedStr string) error {
	var elapsed float64
	if _, err := fmt.Sscanf(elapsedStr, "%f", &elapsed); err != nil {
		return fmt.Errorf("%w: %q: %w", errParseTestEventElapsed, elapsedStr, err)
	}
	if s.packageTestEvents == nil {
		s.packageTestEvents = make(map[string]packageTestEventSpec)
	}
	if _, exists := s.packageTestEvents[importPath]; exists {
		return fmt.Errorf("%w: %q", errPackageTestEventAlreadyRegistered, importPath)
	}
	s.packageTestEvents[importPath] = packageTestEventSpec{testName: testName, elapsed: elapsed}
	return nil
}

func (s *scenarioState) givenModulePackageAt(module, importPathOrSeg, dir string) error {
	s.modulePath = module
	if importPathOrSeg == module || strings.HasPrefix(importPathOrSeg, module+"/") {
		s.pkgImport = importPathOrSeg
	} else {
		s.pkgImport = module + "/" + importPathOrSeg
	}
	s.pkgDir = fsnorm.Canonical(dir)
	if s.modulePackages == nil {
		s.modulePackages = make(map[string]gatetest.PackageListInfo)
	}
	prev := s.modulePackages[s.pkgImport]
	if len(prev.GoFiles) == 0 {
		prev.GoFiles = []string{"pkg.go"}
	}
	prev.Dir = s.pkgDir
	s.modulePackages[s.pkgImport] = prev
	return nil
}

func (s *scenarioState) modulePackagesHasImportSuffix(suffix string) bool {
	for k := range s.modulePackages {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}

func (s *scenarioState) ensureMem() qg.FileOps {
	if s.mem == nil {
		mem := gatetest.NewMemoryFileOps()
		if err := mem.Root("."); err != nil {
			panic("Root scenario MemoryFileOps: " + err.Error())
		}
		s.mem = mem
	}
	return s.mem
}

func (s *scenarioState) ensureStore() *qg.ArtifactStore {
	if s.store == nil {
		s.store = qg.NewArtifactStore()
	}
	return s.store
}

func (s *scenarioState) assertStepPasses() error {
	if s.runErr != nil {
		return fmt.Errorf("%w: %w", errStepPass, s.runErr)
	}
	return nil
}

// assertStepFails only checks that the gate step returned an error. Feature scenarios pair
// this with a second Then step ("the output is …") that asserts diagnostics or CI text, so
// step-specific error types are not required here.
func (s *scenarioState) assertStepFails() error {
	if s.runErr == nil {
		return errStepFail
	}
	return nil
}

func (s *scenarioState) assertConfigurationRejected() error {
	if s.runErr == nil {
		return errConfigReject
	}
	var ve *qg.ValidationError
	if !errors.As(s.runErr, &ve) {
		return fmt.Errorf("%w: got %T %w", errConfigReject, s.runErr, s.runErr)
	}
	return nil
}

func (s *scenarioState) assertOutputEmpty() error {
	progressless := strings.TrimSpace(removeProgressLines(s.output.String()))
	if progressless == "" {
		return nil
	}

	if s.runErr != nil {
		return fmt.Errorf("%w: expected no tool output for failed step, got: %q", errOutputEmpty, s.output.String())
	}
	if s.outputMode == qg.OutputModeVerbose {
		return fmt.Errorf("%w: expected no tool output in verbose mode, got: %q", errOutputEmpty, s.output.String())
	}
	return fmt.Errorf("%w: expected empty output, got %q", errOutputEmpty, s.output.String())
}

func (s *scenarioState) assertOutputEmptyAfterRemovingProgress() error {
	if strings.TrimSpace(removeProgressLines(s.output.String())) != "" {
		return fmt.Errorf(
			"%w: expected empty output when ignoring progress markers, got %q",
			errOutputEmpty,
			s.output.String(),
		)
	}
	return nil
}

func (s *scenarioState) assertOutputIsErrorFixHintDiagnostic() error {
	if s.runErr == nil {
		return errOutputNotDiag
	}
	if err := assertDiagnosticBlocksOnString(s.runErr.Error(), errOutputNotDiag); err != nil {
		return err
	}
	if s.outputMode == qg.OutputModeAgent {
		display := strings.TrimSpace(removeProgressLines(s.output.String()))
		if display == "" {
			return fmt.Errorf("%w: expected diagnostic tuple on display in agent mode", errOutputNotDiag)
		}
		return assertDiagnosticBlocksOnString(display, errOutputNotDiag)
	}
	return nil
}

func (s *scenarioState) assertOutputContainsToolFullOutput() error {
	combined := s.mutationScanVerboseEvidenceForCurrentStep() + s.output.String()
	s.mutationScanVerboseEvidence = ""
	s.mutationScanVerboseEvidenceForStep = ""
	if s.runErr != nil {
		errStr := s.runErr.Error()
		if strings.HasPrefix(errStr, "ERROR:") {
			return fmt.Errorf(
				"%w: verbose display failure must not use ERROR/Fix/Hint wrapping, got: %s",
				errVerboseOutputExpected, errStr,
			)
		}
		if !hasStepFailurePrefix(errStr) {
			return fmt.Errorf(
				"%w: verbose display failure error must start with a step prefix, got: %q",
				errVerboseOutputExpected, errStr,
			)
		}
		if combined == "" {
			return fmt.Errorf(
				"%w: verbose display failure produced no display output (expected tool output to be visible)",
				errVerboseOutputExpected,
			)
		}
		return s.requireAllLinesRecognizableVerbose(combined, s.expectedProgressTitlesForCurrentOutput())
	}
	return s.requireRecognizedVerboseToolOutput(combined)
}

func (s *scenarioState) mutationScanVerboseEvidenceForCurrentStep() string {
	if s.mutationScanVerboseEvidenceForStep != s.stepName {
		return ""
	}
	return s.mutationScanVerboseEvidence
}

func (s *scenarioState) assertArtifactStoreContains(name string) error {
	store := s.ensureStore()
	stepID, found := store.FindArtifact(name)
	if !found {
		return fmt.Errorf("%w: %q", errArtifactMissing, name)
	}
	s.lastArtifact = name
	s.lastArtifactID = stepID
	return nil
}
