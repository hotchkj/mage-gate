// Vision: Compose FakeRunner responses and validate command keys/argv match the harness contracts under test.
//
//nolint:revive // file-length-limit is enforced globally, keep step contract fixtures grouped
package steps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

var (
	errContractMissingArg    = errors.New("missing required arg")
	errContractMissingAnchor = errors.New("missing required ordering anchor")
	errContractArgOrder      = errors.New("expected ordering anchor before arg")
)

const goTestCommand = "go test"

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func requireCommandArgs(inner cmdtest.CommandFunc, required []string) cmdtest.CommandFunc {
	if len(required) == 0 {
		return inner
	}
	req := append([]string(nil), required...)
	return func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
		for _, want := range req {
			if !containsArg(cmd.Args(), want) {
				return fmt.Errorf("%w: want=%q in %v", errContractMissingArg, want, cmd.Args())
			}
		}
		return inner(ctx, cmd, stdout, stderr)
	}
}

// goListIsModuleQuery and requireGoListQualityScopePattern are deleted.
// Quality scope is now asserted by user-facing Then steps.

func argIndex(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

func checkArgOrder(args []string, before string, expectAfter []string) error {
	beforeIdx := argIndex(args, before)
	if beforeIdx < 0 {
		return fmt.Errorf("%w: anchor=%q in %v", errContractMissingAnchor, before, args)
	}
	for _, want := range expectAfter {
		afterIdx := argIndex(args, want)
		if afterIdx < 0 {
			return fmt.Errorf("%w: want=%q in %v", errContractMissingArg, want, args)
		}
		if beforeIdx >= afterIdx {
			return fmt.Errorf("%w: before=%q want=%q in %v", errContractArgOrder, before, want, args)
		}
	}
	return nil
}

func requireArgOrder(inner cmdtest.CommandFunc, before string, after []string) cmdtest.CommandFunc {
	if len(after) == 0 {
		return inner
	}
	expectAfter := append([]string(nil), after...)
	return func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
		if err := checkArgOrder(cmd.Args(), before, expectAfter); err != nil {
			return err
		}
		return inner(ctx, cmd, stdout, stderr)
	}
}

// requireSubsequenceArgOrder requires ordered to appear as a subsequence of cmd.Args (in order).
func requireSubsequenceArgOrder(inner cmdtest.CommandFunc, ordered []string) cmdtest.CommandFunc {
	if len(ordered) == 0 {
		return inner
	}
	want := append([]string(nil), ordered...)
	return func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
		args := cmd.Args()
		pos := 0
		for _, w := range want {
			for pos < len(args) && args[pos] != w {
				pos++
			}
			if pos >= len(args) || args[pos] != w {
				return fmt.Errorf("%w: want %q in order in %v", errContractMissingArg, w, args)
			}
			pos++
		}
		return inner(ctx, cmd, stdout, stderr)
	}
}

func (s *scenarioState) pkgScopeRequiredArgs() []string {
	if s.pkgScopePattern == "" {
		return nil
	}
	return []string{s.pkgScopePattern}
}

func (s *scenarioState) mutationProducerCommands(step string) []string {
	return s.expectedGremlinsCommandsForStep(step)
}

func (s *scenarioState) mutationConsumerCommands() []string {
	return nil
}

func (s *scenarioState) composeRunnerOptions(stepName string) []cmdtest.RunnerOption {
	composers := map[string]func() []cmdtest.RunnerOption{
		stepLint:             s.composeLintOpts,
		stepFormat:           s.composeFormatOpts,
		stepVet:              s.composeVetOpts,
		stepCompile:          s.composeCompileOpts,
		stepTest:             s.composeTestOpts,
		stepQualityInventory: s.composeQualityScopeInventoryOpts,
		stepCoveredTest:      s.composeCoveredTestOpts,
		stepCoverage:         s.composeCoverageOpts,
		stepCrap:             s.composeCrapOpts,
		stepDeadcode:         s.composeDeadcodeOpts,
		stepMarkdownlint:     s.composeMarkdownlintOpts,
		stepMutationScan:     s.composeMutationScanOpts,
		stepMutationSites:    s.composeMutationSitesOpts,
		stepMutationKills:    s.composeMutationKillsOpts,
		stepDuration:         s.composeDurationOpts,
	}

	var opts []cmdtest.RunnerOption
	if fn, ok := composers[stepName]; ok {
		opts = fn()
	}
	return append(opts, s.responses...)
}

func (s *scenarioState) composeLintOpts() []cmdtest.RunnerOption {
	return s.composeGolangciLintOpts(stepLint)
}

func (s *scenarioState) composeFormatOpts() []cmdtest.RunnerOption {
	return s.composeGolangciLintOpts(stepFormat)
}

func (s *scenarioState) composeGolangciLintOpts(step string) []cmdtest.RunnerOption {
	commandKey := s.toolStateForArgs(step)
	required := append(s.pkgScopeRequiredArgs(), s.lintExtraArgs...)
	switch {
	case s.lintDirty:
		return []cmdtest.RunnerOption{
			cmdtest.On(commandKey, requireCommandArgs(lintDirtyResponse(), required)),
		}
	case s.lintClean:
		return []cmdtest.RunnerOption{
			cmdtest.On(commandKey, requireCommandArgs(lintCleanResponse, required)),
		}
	}
	return nil
}

func (s *scenarioState) composeVetOpts() []cmdtest.RunnerOption {
	required := append(s.pkgScopeRequiredArgs(), s.vetExtraArgs...)
	if s.vetIssues {
		return []cmdtest.RunnerOption{
			cmdtest.On("go vet", requireCommandArgs(vetDirtyResponse(), required)),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On("go vet", requireCommandArgs(vetCleanResponse, required)),
	}
}

func (s *scenarioState) composeCompileOpts() []cmdtest.RunnerOption {
	required := append(s.pkgScopeRequiredArgs(), s.compileExtraArgs...)
	if s.compileFails {
		return []cmdtest.RunnerOption{
			cmdtest.On("go build", requireCommandArgs(compileDirtyResponse(), required)),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On("go build", requireCommandArgs(compileCleanResponse, required)),
	}
}

func (s *scenarioState) goListOptsOrDefaultForCoveredTest() []cmdtest.RunnerOption {
	listOpts := s.goListOpts()
	if len(listOpts) == 0 {
		return s.defaultGoListForCoveredTest()
	}
	return listOpts
}

func (s *scenarioState) composeTestOpts() []cmdtest.RunnerOption {
	return s.goTestOpts()
}

func (s *scenarioState) composeQualityScopeInventoryOpts() []cmdtest.RunnerOption {
	return s.goListOptsOrDefaultForCoveredTest()
}

func (s *scenarioState) composeCoveredTestOpts() []cmdtest.RunnerOption {
	return s.goTestOpts()
}

func (s *scenarioState) composeCoverageOpts() []cmdtest.RunnerOption {
	var opts []cmdtest.RunnerOption
	opts = append(opts, s.goTestOpts()...)
	opts = append(opts, s.goToolCoverOpts()...)
	return opts
}

func (s *scenarioState) composeCrapOpts() []cmdtest.RunnerOption {
	listOpts := s.goListOptsOrDefaultForCoveredTest()
	var opts []cmdtest.RunnerOption
	opts = append(opts, listOpts...)
	opts = append(opts, s.goTestOpts()...)
	opts = append(opts, s.goToolCoverOpts()...)
	opts = append(opts, s.gocycloOpts()...)
	return opts
}

func (s *scenarioState) composeDeadcodeOpts() []cmdtest.RunnerOption {
	commandKey := s.toolStateForArgs(stepDeadcode)
	required := append(s.pkgScopeRequiredArgs(), s.deadcodeExtraArgs...)
	if s.deadcodeIssues {
		return []cmdtest.RunnerOption{
			cmdtest.On(commandKey, requireCommandArgs(deadcodeDirtyResponse(), required)),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On(commandKey, requireCommandArgs(deadcodeCleanResponse, required)),
	}
}

func (s *scenarioState) composeMarkdownlintOpts() []cmdtest.RunnerOption {
	commandKey := s.toolStateForArgs(stepMarkdownlint)
	required := append([]string(nil), s.markdownlintExtraArgs...)
	if s.markdownlintIssues {
		return []cmdtest.RunnerOption{
			cmdtest.On(commandKey, requireCommandArgs(markdownlintDirtyResponse(), required)),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On(commandKey, requireCommandArgs(markdownlintCleanResponse, required)),
	}
}

func (s *scenarioState) composeMutationSitesOpts() []cmdtest.RunnerOption {
	return s.composeGremlinsDryRunOpts(stepMutationSites)
}

func (s *scenarioState) composeMutationScanOpts() []cmdtest.RunnerOption {
	return s.composeGremlinsDryRunOpts(stepMutationScan)
}

func (s *scenarioState) composeGremlinsDryRunOpts(step string) []cmdtest.RunnerOption {
	mem := s.ensureMem()
	report := s.buildMutationSitesReport()
	commandKey := s.toolStateForArgs(step)
	opts := s.goListOpts()
	gremlins := gatetest.Gremlins(mem, ".", report)
	mutationArgs := s.mutationPassthroughArgs()
	var inner cmdtest.CommandFunc
	seq, seqErr := s.mutationScanRequiredArgSequence()
	switch {
	case seqErr != nil:
		inner = func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
			return seqErr
		}
	case len(seq) > 0:
		inner = requireSubsequenceArgOrder(gremlins, seq)
	default:
		inner = requireArgOrder(
			requireCommandArgs(gremlins, mutationArgs),
			"--dry-run",
			mutationArgs,
		)
	}
	wrapped := func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
		s.gremlinsDryRunInvocations++
		return inner(ctx, cmd, stdout, stderr)
	}
	return append(opts, cmdtest.On(commandKey, wrapped))
}

func (s *scenarioState) buildMutationSitesReport() []byte {
	if s.mutationExcessive {
		return []byte(
			`{"files":[{"file_name":"pkg/foo.go","mutations":[` +
				`{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"}]}]}`,
		)
	}
	if len(s.qualityScopeTestFilePatterns) > 0 {
		return mutationSitesTestFilePatternReport()
	}
	if s.usesZzBddMutationScope() {
		return s.zzBddMutationSitesReport()
	}
	return []byte(`{"files":[{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]}]}`)
}

func mutationSitesTestFilePatternReport() []byte {
	return []byte(
		`{"files":[` +
			`{"file_name":"pkg/foo.go","mutations":[{"status":"KILLED"}]},` +
			`{"file_name":"pkg/foo_test.go","mutations":[` +
			`{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"}]}` +
			`]}`,
	)
}

func (s *scenarioState) usesZzBddMutationScope() bool {
	const zzBddSub = "zz_bdd_not_default/"
	if _, _, ok := strings.Cut(s.qualityScopePattern, "zz_bdd_not_default"); !ok {
		return false
	}
	for k := range s.modulePackages {
		if _, _, ok := strings.Cut(k, zzBddSub); ok {
			return true
		}
	}
	return false
}

func (s *scenarioState) zzBddMutationSitesReport() []byte {
	if s.zzBddWantsHighCoverageMutationFixture() {
		return s.zzBddMutationCoverageReport()
	}
	if len(s.qualityScopeExcludes) > 0 &&
		s.modulePackagesHasImportSuffix("/zz_bdd_not_default/testutil") {
		main := `{"file_name":"zz_bdd_not_default/pkg/foo.go","mutations":[` +
			`{"status":"KILLED"},{"status":"KILLED"}]}`
		testutil := `{"file_name":"zz_bdd_not_default/testutil/pkg.go","mutations":[` +
			`{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"}]}`
		return []byte(`{"files":[` + main + `,` + testutil + `]}`)
	}
	return []byte(
		`{"files":[{"file_name":"zz_bdd_not_default/pkg/foo.go","mutations":[` +
			`{"status":"KILLED"},{"status":"KILLED"}]}]}`,
	)
}

func (s *scenarioState) zzBddMutationCoverageReport() []byte {
	// BDD mutationcoverage.feature: 4 RUNNABLE + 1 NOT_COVERED = 80%.
	main := `{"file_name":"zz_bdd_not_default/pkg/foo.go","mutations":[` +
		`{"status":"RUNNABLE"},{"status":"RUNNABLE"},{"status":"RUNNABLE"},{"status":"RUNNABLE"},` +
		`{"status":"NOT_COVERED"}]}`
	if len(s.qualityScopeExcludes) > 0 &&
		s.modulePackagesHasImportSuffix("/zz_bdd_not_default/testutil") {
		tu := `{"file_name":"zz_bdd_not_default/testutil/pkg.go","mutations":[` +
			`{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},` +
			`{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}]}`
		return []byte(`{"files":[` + main + `,` + tu + `]}`)
	}
	return []byte(`{"files":[` + main + `]}`)
}

func (s *scenarioState) composeMutationKillsOpts() []cmdtest.RunnerOption {
	mem := s.ensureMem()
	report := s.buildMutationKillsReport()
	commandKey := s.toolStateForArgs(stepMutationKills)
	opts := s.goListOpts()
	gremlins := gatetest.Gremlins(mem, ".", report)
	mutationArgs := s.mutationPassthroughArgs()
	var handler cmdtest.CommandFunc
	seq, seqErr := s.mutationKillsRequiredArgSequence()
	switch {
	case seqErr != nil:
		handler = func(ctx context.Context, cmd cmdrunner.Command, stdout, stderr io.Writer) error {
			return seqErr
		}
	case len(seq) > 0:
		handler = requireSubsequenceArgOrder(gremlins, seq)
	default:
		handler = requireCommandArgs(gremlins, mutationArgs)
	}
	return append(opts, cmdtest.On(commandKey, handler))
}

func (s *scenarioState) buildMutationKillsReport() []byte {
	if len(s.mutationKillsResult) == 0 {
		return []byte(`{"files":[{"file_name":"pkg/foo.go","mutations":[]}]}`)
	}

	var mutations []string
	for status, count := range s.mutationKillsResult {
		for i := 0; i < count; i++ {
			mutations = append(mutations, fmt.Sprintf(`{"status":%q}`, status))
		}
	}
	mutStr := strings.Join(mutations, ",")
	fooFile := fmt.Sprintf(`{"file_name":"pkg/foo.go","mutations":[%s]}`, mutStr)
	files := fooFile
	if s.modulePackagesHasImportSuffix("/zz_bdd_not_default/testutil") {
		var killedOnly []string
		for status, count := range s.mutationKillsResult {
			if status != "KILLED" {
				continue
			}
			for i := 0; i < count; i++ {
				killedOnly = append(killedOnly, `{"status":"KILLED"}`)
			}
		}
		killedStr := strings.Join(killedOnly, ",")
		files += fmt.Sprintf(`,{"file_name":"zz_bdd_not_default/testutil/pkg.go","mutations":[%s]}`, killedStr)
	}
	if len(s.qualityScopeTestFilePatterns) > 0 {
		return []byte(fmt.Sprintf(
			`{"files":[%s,{"file_name":"pkg/foo_test.go","mutations":[{"status":"NOT_COVERED"}]}]}`,
			files,
		))
	}
	return []byte(fmt.Sprintf(`{"files":[%s]}`, files))
}

func (s *scenarioState) composeDurationOpts() []cmdtest.RunnerOption {
	return s.goTestOpts()
}

func (s *scenarioState) goTestOpts() []cmdtest.RunnerOption {
	mem := s.ensureMem()
	required := append(s.pkgScopeRequiredArgs(), s.testExtraArgs...)
	if s.testFails {
		return []cmdtest.RunnerOption{
			cmdtest.On(goTestCommand, requireCommandArgs(testDirtyResponse(), required)),
		}
	}
	if s.slowTests {
		return []cmdtest.RunnerOption{
			cmdtest.On(goTestCommand, requireCommandArgs(slowTestResponse(mem), required)),
		}
	}
	if s.fastTestsSlowWall {
		return []cmdtest.RunnerOption{
			cmdtest.On(goTestCommand, requireCommandArgs(fastTestsSlowPackageWallClockResponse(mem), required)),
		}
	}
	if len(s.packageTestEvents) > 0 {
		return []cmdtest.RunnerOption{
			cmdtest.On(goTestCommand, requireCommandArgs(s.multiPackageTestResponse(mem), required)),
		}
	}
	if s.testCovExplicit {
		return []cmdtest.RunnerOption{
			cmdtest.On(
				goTestCommand,
				requireCommandArgs(gatetest.GoTestPassWithCoverage(mem, testPkgName, s.testCovPercent), required),
			),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On(goTestCommand, requireCommandArgs(gatetest.GoTestPass(mem, testPkgName), required)),
	}
}

func slowTestResponse(fileOps gatetest.FileOpsWriter) cmdtest.CommandFunc {
	const slowElapsed = 5.0
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		if err := writeDurationTestEvents(stdout, testPkgName, "TestSlow", slowElapsed, slowElapsed); err != nil {
			return err
		}
		return writeCoverprofile(cmd, fileOps)
	}
}

func fastTestsSlowPackageWallClockResponse(fileOps gatetest.FileOpsWriter) cmdtest.CommandFunc {
	const (
		fastElapsed = 0.01
		slowWall    = 5.0
	)
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		if err := writeDurationTestEvents(stdout, testPkgName, "TestFast", fastElapsed, slowWall); err != nil {
			return err
		}
		return writeCoverprofile(cmd, fileOps)
	}
}

func writeDurationTestEvents(stdout io.Writer, pkg, test string, testElapsed, packageElapsed float64) error {
	lines := []string{
		fmt.Sprintf(`{"Action":"run","Package":%q,"Test":%q}`, pkg, test),
		fmt.Sprintf(`{"Action":"pass","Package":%q,"Test":%q,"Elapsed":%g}`, pkg, test, testElapsed),
		fmt.Sprintf(`{"Action":"pass","Package":%q,"Elapsed":%g}`, pkg, packageElapsed),
	}
	_, err := io.WriteString(stdout, strings.Join(lines, "\n")+"\n")
	return err
}

func writeCoverprofile(cmd cmdrunner.Command, fileOps gatetest.FileOpsWriter) error {
	p := gatetest.CoverprofilePathFromArgs(cmd.Args())
	if p == "" {
		return nil
	}
	return gatetest.WriteMinimalCoverprofile(fileOps, p)
}

func writeScenarioCoverprofile(fileOps gatetest.FileOpsWriter, cmd cmdrunner.Command, coveragePercent int) error {
	fn := gatetest.GoTestPassWithCoverage(fileOps, "unused", coveragePercent)
	return fn(context.Background(), cmd, io.Discard, io.Discard)
}

func (s *scenarioState) multiPackageTestResponse(fileOps gatetest.FileOpsWriter) cmdtest.CommandFunc {
	const (
		defaultTest    = "TestPass"
		defaultElapsed = 0.01
	)
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		if len(s.modulePackages) == 0 {
			return fmt.Errorf("%w: register module packages before package test events", errModuleSourceNotDerivable)
		}
		importPaths := make([]string, 0, len(s.modulePackages))
		for importPath := range s.modulePackages {
			importPaths = append(importPaths, importPath)
		}
		sort.Strings(importPaths)
		for _, importPath := range importPaths {
			testName := defaultTest
			elapsed := defaultElapsed
			if spec, ok := s.packageTestEvents[importPath]; ok {
				testName = spec.testName
				elapsed = spec.elapsed
			}
			if err := writeDurationTestEvents(stdout, importPath, testName, elapsed, elapsed); err != nil {
				return err
			}
		}
		if s.testCovExplicit {
			return writeScenarioCoverprofile(fileOps, cmd, s.testCovPercent)
		}
		return writeCoverprofile(cmd, fileOps)
	}
}

func (s *scenarioState) goToolCoverOpts() []cmdtest.RunnerOption {
	toolCovPct := float64(s.testCovPercent)
	if !s.testCovExplicit {
		// Align with [gatetest.GoTestPass] + its minimal profile (full statement coverage) when the scenario
		// did not fix a percent via "the codebase has N% test coverage".
		toolCovPct = 100.0
	}
	if len(s.gocycloScores) > 0 {
		funcs := make(map[string]float64)
		for name := range s.gocycloScores {
			key := fmt.Sprintf("%s:%d:\t%s", crapCoverFile, crapCoverLine, name)
			funcs[key] = toolCovPct
		}
		return []cmdtest.RunnerOption{
			cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(funcs, toolCovPct)),
		}
	}
	return []cmdtest.RunnerOption{
		cmdtest.On("go tool cover", gatetest.GoToolCover(toolCovPct)),
	}
}

func (s *scenarioState) goListOpts() []cmdtest.RunnerOption {
	if s.modulePath == "" {
		return nil
	}
	var pkgs map[string]gatetest.PackageListInfo
	switch {
	case len(s.modulePackages) > 0:
		pkgs = s.modulePackages
	case s.pkgImport != "" && s.pkgDir != "":
		pkgs = map[string]gatetest.PackageListInfo{s.pkgImport: gatetest.DirOnly(s.pkgDir)}
	default:
		return nil
	}
	modDir := s.moduleSourceRoot()
	return []cmdtest.RunnerOption{
		cmdtest.On("go list", gatetest.GoList(s.modulePath, modDir, pkgs)),
	}
}

// defaultGoListForCoveredTest supplies a go list handler when scenarios run CoveredTest
// without the module Background step (e.g. coverage.feature).
func (s *scenarioState) defaultGoListForCoveredTest() []cmdtest.RunnerOption {
	pkgs := map[string]gatetest.PackageListInfo{
		testPkgName: gatetest.DirOnly("/mod/pkg"),
	}
	return []cmdtest.RunnerOption{
		cmdtest.On("go list", gatetest.GoList("", "", pkgs)),
	}
}

func (s *scenarioState) gocycloOpts() []cmdtest.RunnerOption {
	if len(s.gocycloScores) > 0 {
		commandKey := s.toolStateForArgs(stepCrap)
		return []cmdtest.RunnerOption{
			cmdtest.On(commandKey, requireCommandArgs(gatetest.Gocyclo(s.gocycloScores), s.crapExtraArgs)),
		}
	}
	return nil
}

func keysForCall(c cmdrunner.Command) []string {
	name := c.Name()
	a0, a1 := c.Arg(0), c.Arg(1)
	var keys []string
	if a0 != "" && a1 != "" {
		keys = append(keys, name+" "+a0+" "+a1)
	}
	if a0 != "" {
		keys = append(keys, name+" "+a0)
	}
	keys = append(keys, name)
	return keys
}

func callsSatisfyContract(calls []cmdrunner.Command, want []string) bool {
	for _, wantKey := range want {
		found := false
		for _, c := range calls {
			for _, k := range keysForCall(c) {
				if k == wantKey {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *scenarioState) expectedLintCommandsForStep(step string) []string {
	state, _ := s.localToolState(step)
	return gatetest.ResolverExpectedKeys(
		"golangci-lint",
		s.expectedToolSpecForStep(step),
		gatetest.LocalToolState(state),
	)
}

func (s *scenarioState) expectedDeadcodeCommands() []string {
	state, _ := s.localToolState(stepDeadcode)
	return gatetest.ResolverExpectedKeys(
		"deadcode",
		s.expectedToolSpecForStep(stepDeadcode),
		gatetest.LocalToolState(state),
	)
}

func (s *scenarioState) expectedMarkdownlintCommands() []string {
	state, _ := s.localToolState(stepMarkdownlint)
	return gatetest.ResolverExpectedKeys(
		"gomarklint",
		s.expectedToolSpecForStep(stepMarkdownlint),
		gatetest.LocalToolState(state),
	)
}

func (s *scenarioState) expectedGremlinsCommandsForStep(step string) []string {
	state, _ := s.localToolState(step)
	return gatetest.ResolverExpectedKeys(
		"gremlins",
		s.expectedToolSpecForStep(step),
		gatetest.LocalToolState(state),
	)
}

func (s *scenarioState) expectedGocycloCommands() []string {
	state, _ := s.localToolState(stepCrap)
	return gatetest.ResolverExpectedKeys(
		"gocyclo",
		s.expectedToolSpecForStep(stepCrap),
		gatetest.LocalToolState(state),
	)
}

func (s *scenarioState) expectedStaticToolchainCommands(stepName string) ([]string, bool) {
	switch stepName {
	case stepVet:
		return gatetest.VetCommandKeys(), true
	case stepCompile:
		return gatetest.CompileCommandKeys(), true
	case stepTest:
		return gatetest.TestCommandKeys(), true
	case stepQualityInventory:
		return []string{"go list -e"}, true
	case stepCoveredTest:
		return gatetest.TestCommandKeys(), true
	case stepCoverage:
		return []string{"go tool cover"}, true
	case stepDuration:
		return gatetest.DurationCommandKeys(), true
	case stepMutationCoverage:
		return nil, true
	default:
		return nil, false
	}
}

// expectedCommandsForStep returns all FakeRunner keys that must be dispatched for the step.
// This includes helper commands (go list, go tool cover) where applicable.
func (s *scenarioState) expectedCommandsForStep(stepName string) ([]string, error) {
	if keys, ok := s.expectedStaticToolchainCommands(stepName); ok {
		return keys, nil
	}
	switch stepName {
	case stepLint:
		return s.expectedLintCommandsForStep(stepLint), nil
	case stepFormat:
		return s.expectedLintCommandsForStep(stepFormat), nil
	case stepDeadcode:
		return s.expectedDeadcodeCommands(), nil
	case stepMarkdownlint:
		return s.expectedMarkdownlintCommands(), nil
	case stepCrap:
		return append([]string{"go list -m", "go tool cover"}, s.expectedGocycloCommands()...), nil
	case stepMutationScan:
		return s.mutationProducerCommands(stepMutationScan), nil
	case stepMutationSites, stepMutationCoverage:
		// Consumer-only steps: gremlins ran during a prior scan; no new subprocess contract.
		return s.mutationConsumerCommands(), nil
	case stepMutationKills:
		return s.mutationProducerCommands(stepMutationKills), nil
	}
	return nil, fmt.Errorf("%w: %q", errUnsupportedStep, stepName)
}

func (s *scenarioState) validateContractCalls(calls []cmdrunner.Command, stepName string) error {
	s.recordedCalls = calls
	s.allDispatchedCalls = append(s.allDispatchedCalls, calls...)
	// Store step-specific calls for multi-step scenario assertions (tool resolution checks).
	s.stepCallsMap[stepName] = append(s.stepCallsMap[stepName], append([]cmdrunner.Command(nil), calls...))
	expected, expectedErr := s.expectedCommandsForStep(stepName)
	if expectedErr != nil {
		return fmt.Errorf("%w: %w", errContractFail, expectedErr)
	}
	if len(expected) == 0 {
		if len(calls) != 0 {
			return fmt.Errorf("%w: step %q expected no commands, got %v", errContractFail, stepName, calls)
		}
		return nil
	}
	if !callsSatisfyContract(calls, expected) {
		return fmt.Errorf(
			"%w: step %q want %v, got %v",
			errContractFail, stepName, expected, calls,
		)
	}
	return nil
}
