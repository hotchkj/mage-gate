// Vision: Core compile/test/deadcode harness implementations—command assembly, stores, and gatecheck handoff.
// Coverage lives in step_coverage.go; one test run still feeds duration, coverage, and CRAP.
package harness

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// StepDeadcode runs deadcode: add "-test" in deadcodeArgs for test-inclusive reachability, omit for production-only.
// Uses a local binary when it matches toolSpec, otherwise `go run` with that spec.
func (h *StepRunner) StepDeadcode(ctx context.Context, deadcodeSpec string, deadcodeArgs []string) error {
	pkgs := resolvePackages(h.packages)
	if h.toolResolver == nil {
		return fmt.Errorf("%w: ToolResolver is required", ErrDeadcodeFailed)
	}

	extraArgs := append([]string{}, deadcodeArgs...)
	extraArgs = append(extraArgs, pkgs)
	binary, args, err := h.toolResolver.ResolveToolCommand(ctx, "deadcode", deadcodeSpec, extraArgs)
	if err != nil {
		return fmt.Errorf("%w: resolve deadcode command: %w", ErrDeadcodeFailed, err)
	}

	result, err := h.runCommand(ctx, h.root, binary, args...)
	if err != nil {
		return fmt.Errorf("%w: deadcode command: %w", ErrDeadcodeFailed, err)
	}
	if trimmed := strings.TrimSpace(result.Stdout); trimmed != "" {
		return fmt.Errorf("%w: unreachable functions detected\n%s", ErrDeadcodeFailed, trimmed)
	}
	return nil
}

// StepCompile runs go build with the given extra arguments and harness package scope.
func (h *StepRunner) StepCompile(ctx context.Context, compileArgs []string) error {
	pkgs := resolvePackages(h.packages)
	args := []string{"build"}
	args = append(args, compileArgs...)
	args = append(args, pkgs)
	if _, err := h.runCommand(ctx, h.root, "go", args...); err != nil {
		return fmt.Errorf("%w: go build: %w", ErrCompileFailed, err)
	}
	return nil
}

// StepTest runs go test once with all flags needed for coverage and duration.
// It produces coverage.out and test-events.jsonl in one pass.
func (h *StepRunner) StepTest(
	ctx context.Context,
	coverpkg string,
	short bool,
	tags string,
	testArgs []string,
) (err error) {
	pkgs := resolvePackages(h.packages)

	_, coverageFileOpsPath, eventsFileOpsPath, eventsFile, setupErr := h.setupTestArtifacts()
	if setupErr != nil {
		return setupErr
	}
	defer func() {
		if eventsFile == nil {
			return
		}
		err = joinStepTestCloseErr(err, eventsFile.Close())
		eventsFile = nil
	}()

	var coverprofileCommandPath string
	if coverpkg != "" {
		var cmdPathErr error
		coverprofileCommandPath, cmdPathErr = h.artifacts.CommandPath("coverage.out")
		if cmdPathErr != nil {
			return fmt.Errorf("%w: coverprofile command path: %w", ErrTestFailed, cmdPathErr)
		}
	}

	bta := buildTestArgs(pkgs, coverprofileCommandPath, coverpkg, short, tags, testArgs)
	if runErr := h.runTestCommand(ctx, eventsFile, bta...); runErr != nil {
		return runErr
	}
	if cerr := eventsFile.Close(); cerr != nil {
		eventsFile = nil
		return joinStepTestCloseErr(nil, cerr)
	}
	eventsFile = nil

	return h.storeTestArtifacts(coverageFileOpsPath, eventsFileOpsPath, coverpkg)
}

// StepCoveredTest runs go test with -coverprofile and -coverpkg. The go test invocation uses
// h.packages as the run target; command scope supplies -coverpkg and quality-scope build tags.
func (h *StepRunner) StepCoveredTest(
	ctx context.Context,
	commandScope *gatecheck.QualityScopeCommandScope,
	short bool,
	testArgs []string,
) error {
	if commandScope == nil {
		return fmt.Errorf("%w: quality scope command scope is nil", ErrTestFailed)
	}
	tags, testArgs := mergeBuildTags(commandScope.TagsCSV(), testArgs)
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTestFailed, err)
	}
	return h.StepTest(ctx, coverpkg, short, tags, testArgs)
}

func (h *StepRunner) setupTestArtifacts() (
	artifactLogicalDir, coverageFileOpsPath, eventsFileOpsPath string,
	eventsFile io.WriteCloser, err error,
) {
	artifactLogicalDir = h.artifacts.Dir()
	if mkdirErr := h.fileOps.MkdirAll(artifactLogicalDir, artifactDirPerm); mkdirErr != nil {
		return "", "", "", nil, fmt.Errorf("%w: create artifact dir: %w", ErrTestFailed, mkdirErr)
	}

	coverageFileOpsPath, err = h.artifacts.FileOpsPath("coverage.out")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("%w: coverage artifact path: %w", ErrTestFailed, err)
	}
	eventsFileOpsPath, err = h.artifacts.FileOpsPath("test-events.jsonl")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("%w: test events artifact path: %w", ErrTestFailed, err)
	}

	eventsFile, err = h.fileOps.CreateFile(eventsFileOpsPath)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("%w: open test events file: %w", ErrTestFailed, err)
	}

	return artifactLogicalDir, coverageFileOpsPath, eventsFileOpsPath, eventsFile, nil
}

// buildTestArgs assembles argv for `go test`. When coverpkg is non-empty, coverprofileCommandPath is
// the value after -coverprofile= and must come from [artifactPaths.CommandPath] so tools resolve it
// against command cwd independently of FileOps projections.
func buildTestArgs(
	pkgs, coverprofileCommandPath, coverpkg string,
	short bool,
	tags string,
	consumerTestArgs []string,
) []string {
	tags, consumerTestArgs = mergeBuildTags(tags, consumerTestArgs)
	testArgs := []string{"test", pkgs, "-json"}

	if coverpkg != "" {
		testArgs = append(testArgs,
			"-coverprofile="+coverprofileCommandPath,
			"-coverpkg="+coverpkg,
		)
	}

	if short {
		testArgs = append(testArgs, "-short")
	}

	if tags != "" {
		testArgs = append(testArgs, "-tags="+tags)
	}

	testArgs = append(testArgs, stripCountFlag(consumerTestArgs)...)
	testArgs = append(testArgs, "-count=1")
	return testArgs
}

// stripCountFlag removes any -count or -count=N flags from args.
// -count=1 is appended unconditionally after consumer args to enforce the cache-bypass invariant.
//
// Implemented as a prefix walk over a shrinking head slice (no index + for-post increment),
// so Gremlins INCREMENT_DECREMENT cannot turn a paired "skip value" bump into a non-terminating loop.
func stripCountFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "-count" || arg == "--count":
			args = args[1:]
			if len(args) > 0 {
				args = args[1:] // drop value token after bare -count / --count
			}
		case strings.HasPrefix(arg, "-count=") || strings.HasPrefix(arg, "--count="):
			args = args[1:]
		default:
			out = append(out, arg)
			args = args[1:]
		}
	}
	return out
}

// runTestCommand captures `go test -json` to eventsFile; writes stdout even on go test failure (partial events).
func (h *StepRunner) runTestCommand(ctx context.Context, eventsFile io.WriteCloser, args ...string) error {
	result, runErr := h.runCommand(ctx, h.root, "go", args...)
	if _, writeErr := io.WriteString(eventsFile, result.Stdout); writeErr != nil {
		return fmt.Errorf("%w: write test events: %w", ErrTestFailed, writeErr)
	}
	if runErr != nil {
		return fmt.Errorf("%w: go test: %w", ErrTestFailed, runErr)
	}
	return nil
}

func (h *StepRunner) storeTestArtifacts(coverageFileOpsPath, eventsFileOpsPath, coverpkg string) error {
	if coverpkg != "" {
		coverageData, readErr := h.fileOps.ReadFile(coverageFileOpsPath)
		if readErr != nil {
			return fmt.Errorf("%w: read coverage profile: %w", ErrTestFailed, readErr)
		}
		covProv := Provenance{StepID: h.stepID, Tool: "go test -coverprofile", Packages: h.packages}
		if writeErr := h.store.Write(h.stepID, "coverage.out", coverageData, covProv); writeErr != nil {
			return fmt.Errorf("%w: store coverage profile: %w", ErrTestFailed, writeErr)
		}
	}

	eventsData, eventsReadErr := h.fileOps.ReadFile(eventsFileOpsPath)
	if eventsReadErr != nil {
		return fmt.Errorf("%w: read test events: %w", ErrTestFailed, eventsReadErr)
	}
	eventsProv := Provenance{StepID: h.stepID, Tool: "go test -json", Packages: h.packages}
	if eventsWriteErr := h.store.Write(h.stepID, "test-events.jsonl", eventsData, eventsProv); eventsWriteErr != nil {
		return fmt.Errorf("%w: store test events: %w", ErrTestFailed, eventsWriteErr)
	}

	return nil
}
