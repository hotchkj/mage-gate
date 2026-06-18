// Vision: Functional options force explicit thresholds (no silent defaults).
// Configuration is always deliberate, never guessed.
package gate

import (
	"fmt"
	"math"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

func validateFiniteFloat64(step string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return newValidationError(step, "value must be finite (not NaN or ±Inf)", ErrInvalidOption)
	}
	return nil
}

// TestOption configures the Test step.
type TestOption func(*testConfig)

type testConfig struct {
	testArgs []string
}

func defaultTestConfig() testConfig {
	return testConfig{}
}

// CoverageThreshold is a required typed input for Coverage.
type CoverageThreshold struct {
	minPercent float64
	set        bool
}

// MinPercent sets the minimum coverage percentage (0-100 inclusive).
// Zero disables the coverage threshold check: the gate passes regardless of reported coverage.
func MinPercent(percent float64) CoverageThreshold {
	return CoverageThreshold{minPercent: percent, set: true}
}

func validateMinPercent(threshold CoverageThreshold) error {
	if !threshold.set {
		return newValidationError(
			"coverage",
			"MinPercent is required — pass gate.MinPercent(n) where 0 disables the check",
			ErrInvalidOption,
		)
	}
	if err := validateFiniteFloat64("coverage", threshold.minPercent); err != nil {
		return err
	}
	if threshold.minPercent < 0 || threshold.minPercent > 100 {
		return newValidationError(
			"coverage",
			fmt.Sprintf("MinPercent must be between 0 and 100, got %f", threshold.minPercent),
			ErrInvalidOption,
		)
	}
	return nil
}

// CrapOption configures the Crap step.
type CrapOption func(*crapConfig)

type crapConfig struct {
	crapArgs []string
}

func defaultCrapConfig() crapConfig {
	return crapConfig{}
}

// CrapArgs appends additional argv tokens to the complexity tool invocation in [Crap],
// after the step's fixed command prefix.
func CrapArgs(args ...string) CrapOption {
	return func(c *crapConfig) {
		c.crapArgs = append(c.crapArgs, args...)
	}
}

// GocycloToolValue is a required typed input for Crap.
type GocycloToolValue struct {
	spec string
	set  bool
}

// GocycloToolSpec sets the required pinned gocyclo tool spec for Crap.
func GocycloToolSpec(spec string) GocycloToolValue {
	return GocycloToolValue{spec: spec, set: true}
}

func validateGocycloToolSpec(tool GocycloToolValue) error {
	if !tool.set {
		return newValidationError(
			"crap",
			"GocycloToolSpec is required — pass gate.GocycloToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if tool.spec == "" {
		return newValidationError(
			"crap",
			"GocycloToolSpec is required — pass gate.GocycloToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if err := cmdrunner.ValidateToolSpec(tool.spec); err != nil {
		return newValidationError(
			"crap",
			fmt.Sprintf("GocycloToolSpec must be in format 'package@version', got %q", tool.spec),
			ErrInvalidOption,
		)
	}
	return nil
}

// CrapThreshold is a required typed input for Crap.
type CrapThreshold struct {
	maxScore float64
	set      bool
}

// MaxScore sets the maximum CRAP score threshold.
func MaxScore(score float64) CrapThreshold {
	return CrapThreshold{maxScore: score, set: true}
}

func validateMaxScore(threshold CrapThreshold) error {
	if !threshold.set {
		return newValidationError(
			"crap",
			"MaxScore is required — pass gate.MaxScore(n) where n > 0",
			ErrInvalidOption,
		)
	}
	if err := validateFiniteFloat64("crap", threshold.maxScore); err != nil {
		return err
	}
	if threshold.maxScore <= 0 {
		return newValidationError(
			"crap",
			fmt.Sprintf("Crap threshold must be positive (got %f)", threshold.maxScore),
			ErrInvalidOption,
		)
	}
	return nil
}

// DurationThreshold is a required typed input for Duration.
type DurationThreshold struct {
	maxSeconds float64
	set        bool
}

// MaxSeconds sets the duration threshold in seconds.
func MaxSeconds(seconds float64) DurationThreshold {
	return DurationThreshold{maxSeconds: seconds, set: true}
}

func validateMaxSeconds(threshold DurationThreshold) error {
	if !threshold.set {
		return newValidationError(
			"duration",
			"MaxSeconds is required — pass gate.MaxSeconds(n) where n > 0",
			ErrInvalidOption,
		)
	}
	if err := validateFiniteFloat64("duration", threshold.maxSeconds); err != nil {
		return err
	}
	if threshold.maxSeconds <= 0 {
		return newValidationError(
			"duration",
			fmt.Sprintf("duration threshold must be positive (got %f)", threshold.maxSeconds),
			ErrInvalidOption,
		)
	}
	return nil
}

// LintOption configures lint toolchain construction via [NewLintToolchain].
type LintOption func(*lintConfig)

type lintConfig struct {
	customGCLPath  string
	customLintSpec string
	lintArgs       []string
}

// LintConfigValue is a required typed input for Lint.
type LintConfigValue struct {
	path string
	set  bool
}

// LintConfig sets the required golangci-lint config path for [NewLintToolchain].
func LintConfig(path string) LintConfigValue {
	return LintConfigValue{path: path, set: true}
}

// LintToolValue is a required typed input for Lint.
type LintToolValue struct {
	spec string
	set  bool
}

// LintToolSpec sets the required pinned golangci-lint tool spec for [NewLintToolchain].
func LintToolSpec(spec string) LintToolValue {
	return LintToolValue{spec: spec, set: true}
}

// CustomGCL sets the path to a replace-style golangci-lint module used when building a custom lint binary.
// Requires [CustomLintToolSpec] so the builder is explicitly pinned.
func CustomGCL(path string) LintOption {
	return func(c *lintConfig) {
		c.customGCLPath = path
	}
}

// CustomLintToolSpec sets the go module path for "go run … custom" when building that binary; requires [CustomGCL].
func CustomLintToolSpec(spec string) LintOption {
	return func(c *lintConfig) {
		c.customLintSpec = spec
	}
}

// LintArgs appends additional argv tokens after the fixed golangci-lint subcommand prefix
// in [Lint] ("run …") and [Format] ("fmt …").
func LintArgs(args ...string) LintOption {
	return func(c *lintConfig) {
		c.lintArgs = append(c.lintArgs, args...)
	}
}

func validateLintInputs(configPath LintConfigValue, lintToolSpec LintToolValue) error {
	if !configPath.set {
		return newValidationError("lint", "LintConfig path is required", ErrLintConfigRequired)
	}
	if configPath.path == "" {
		return newValidationError("lint", "LintConfig path is required", ErrLintConfigRequired)
	}
	if !lintToolSpec.set {
		return newValidationError("lint", "LintToolSpec is required — pass gate.LintToolSpec(spec)", ErrInvalidOption)
	}
	if lintToolSpec.spec == "" {
		return newValidationError("lint", "LintToolSpec is required — pass gate.LintToolSpec(spec)", ErrInvalidOption)
	}
	if err := cmdrunner.ValidateToolSpec(lintToolSpec.spec); err != nil {
		return newValidationError(
			"lint",
			fmt.Sprintf("LintToolSpec must be in format 'package@version', got %q", lintToolSpec.spec),
			ErrInvalidOption,
		)
	}
	return nil
}

// DeadcodeOption configures the Deadcode step.
type DeadcodeOption func(*deadcodeConfig)

type deadcodeConfig struct {
	args []string
}

func defaultDeadcodeConfig() deadcodeConfig {
	return deadcodeConfig{}
}

// DeadcodeArgs appends additional argv tokens to deadcode in [Deadcode],
// after the step's fixed command prefix.
func DeadcodeArgs(args ...string) DeadcodeOption {
	return func(c *deadcodeConfig) {
		c.args = append(c.args, args...)
	}
}

// DeadcodeToolValue is a required typed input for Deadcode.
type DeadcodeToolValue struct {
	spec string
	set  bool
}

// DeadcodeToolSpec sets the required pinned deadcode tool spec for Deadcode.
func DeadcodeToolSpec(spec string) DeadcodeToolValue {
	return DeadcodeToolValue{spec: spec, set: true}
}

// MarkdownLintOption configures the MarkdownLint step.
type MarkdownLintOption func(*markdownlintConfig)

type markdownlintConfig struct {
	args []string
}

func defaultMarkdownlintConfig() markdownlintConfig {
	return markdownlintConfig{}
}

// MarkdownLintArgs appends additional argv tokens to gomarklint in [MarkdownLint].
// Pass config path via MarkdownLintArgs("--config", path); the step adds no owned flags.
func MarkdownLintArgs(args ...string) MarkdownLintOption {
	return func(c *markdownlintConfig) {
		c.args = append(c.args, args...)
	}
}

// MarkdownLintToolValue is a required typed input for MarkdownLint.
type MarkdownLintToolValue struct {
	spec string
	set  bool
}

// MarkdownLintToolSpec sets the required pinned gomarklint tool spec for MarkdownLint.
func MarkdownLintToolSpec(spec string) MarkdownLintToolValue {
	return MarkdownLintToolValue{spec: spec, set: true}
}

func validateMarkdownLintToolSpec(tool MarkdownLintToolValue) error {
	if !tool.set {
		return newValidationError(
			"markdownlint",
			"MarkdownLintToolSpec is required — pass gate.MarkdownLintToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if tool.spec == "" {
		return newValidationError(
			"markdownlint",
			"MarkdownLintToolSpec is required — pass gate.MarkdownLintToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if err := cmdrunner.ValidateToolSpec(tool.spec); err != nil {
		return newValidationError(
			"markdownlint",
			fmt.Sprintf("MarkdownLintToolSpec must be in format 'package@version', got %q", tool.spec),
			ErrInvalidOption,
		)
	}
	return nil
}

func validateDeadcodeToolSpec(deadcodeSpec DeadcodeToolValue) error {
	if !deadcodeSpec.set {
		return newValidationError(
			"deadcode",
			"DeadcodeToolSpec is required — pass gate.DeadcodeToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if deadcodeSpec.spec == "" {
		return newValidationError(
			"deadcode",
			"DeadcodeToolSpec is required — pass gate.DeadcodeToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if err := cmdrunner.ValidateToolSpec(deadcodeSpec.spec); err != nil {
		return newValidationError(
			"deadcode",
			fmt.Sprintf("DeadcodeToolSpec must be in format 'package@version', got %q", deadcodeSpec.spec),
			ErrInvalidOption,
		)
	}
	return nil
}

// MutationOption configures [MutationRunner.Scan], [MutationRunner.Kill], and [MutationKills].
type MutationOption func(*mutationConfig)

type mutationConfig struct {
	mutationArgs []string
}

func defaultMutationConfig() mutationConfig {
	return mutationConfig{}
}

// GremlinsToolValue is a required typed input for gremlins-backed mutation steps ([MutationRunner], [MutationKills]).
type GremlinsToolValue struct {
	spec string
	set  bool
}

// GremlinsToolSpec sets the required pinned gremlins tool spec for [MutationRunner] and [MutationKills].
func GremlinsToolSpec(spec string) GremlinsToolValue {
	return GremlinsToolValue{spec: spec, set: true}
}

func validateGremlinsToolSpec(step string, tool GremlinsToolValue) error {
	if !tool.set {
		return newValidationError(
			step,
			"GremlinsToolSpec is required — pass gate.GremlinsToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if tool.spec == "" {
		return newValidationError(
			step,
			"GremlinsToolSpec is required — pass gate.GremlinsToolSpec(spec)",
			ErrInvalidOption,
		)
	}
	if err := cmdrunner.ValidateToolSpec(tool.spec); err != nil {
		return newValidationError(
			step,
			fmt.Sprintf("GremlinsToolSpec must be in format 'package@version', got %q", tool.spec),
			ErrInvalidOption,
		)
	}
	return nil
}

// MutationSitesThreshold is a required typed input for the [MutationSites] check (MaxSites).
type MutationSitesThreshold struct {
	maxSites int
	set      bool
}

// MaxSites sets the mutation sites threshold.
func MaxSites(sites int) MutationSitesThreshold {
	return MutationSitesThreshold{maxSites: sites, set: true}
}

func validateMaxSites(threshold MutationSitesThreshold) error {
	if !threshold.set {
		return newValidationError(
			"mutationsites",
			"MaxSites is required — pass gate.MaxSites(n) where n > 0",
			ErrInvalidOption,
		)
	}
	if threshold.maxSites <= 0 {
		return newValidationError(
			"mutationsites",
			fmt.Sprintf("mutation sites threshold must be positive (got %d)", threshold.maxSites),
			ErrInvalidOption,
		)
	}
	return nil
}

// MutationArgs appends additional argv tokens to the mutation engine invocation for
// [MutationRunner] and [MutationKills], after the fixed "unleash -o <report> --coverpkg <packages>" prefix.
// Step-controlled flags such as sites dry-run remain first.
func MutationArgs(args ...string) MutationOption {
	return func(c *mutationConfig) {
		c.mutationArgs = append(c.mutationArgs, args...)
	}
}

// MinKillRateThreshold is a required typed input for MutationKills and [MutationKillRate].
type MinKillRateThreshold struct {
	minPercent int
	set        bool
}

// MinKillRate sets the minimum mutation kill rate percentage (0-100 inclusive, integer).
// Zero disables the kill rate threshold check: the gate passes regardless of reported kill rate.
func MinKillRate(percent int) MinKillRateThreshold {
	return MinKillRateThreshold{minPercent: percent, set: true}
}

func validateMinKillRate(threshold MinKillRateThreshold) error {
	if !threshold.set {
		return newValidationError(
			"mutationkills",
			"MinKillRate is required — pass gate.MinKillRate(n) where 0 disables the check",
			ErrInvalidOption,
		)
	}
	if threshold.minPercent < 0 || threshold.minPercent > 100 {
		return newValidationError(
			"mutationkills",
			fmt.Sprintf("MinKillRate must be between 0 and 100, got %d", threshold.minPercent),
			ErrInvalidOption,
		)
	}
	return nil
}

// MutationCoverageThreshold is a required typed input for MutationCoverage.
type MutationCoverageThreshold struct {
	minPercent int
	set        bool
}

// MinMutationCoverage sets the minimum mutation coverage percentage (0-100 inclusive, integer).
// Zero disables the mutation coverage threshold check: the gate passes regardless of reported coverage.
func MinMutationCoverage(percent int) MutationCoverageThreshold {
	return MutationCoverageThreshold{minPercent: percent, set: true}
}

func validateMinMutationCoverage(threshold MutationCoverageThreshold) error {
	if !threshold.set {
		return newValidationError(
			"mutationcoverage",
			"MinMutationCoverage is required — pass gate.MinMutationCoverage(n) where 0 disables the check",
			ErrInvalidOption,
		)
	}
	if threshold.minPercent < 0 || threshold.minPercent > 100 {
		return newValidationError(
			"mutationcoverage",
			fmt.Sprintf("MinMutationCoverage must be between 0 and 100, got %d", threshold.minPercent),
			ErrInvalidOption,
		)
	}
	return nil
}

// CompileOption configures the [Compile] step.
type CompileOption func(*compileConfig)

type compileConfig struct {
	compileArgs []string
}

// CompileArgs appends additional argv tokens to go build in [Compile],
// after the step's fixed command prefix.
// Example: qg.Compile(..., qg.CompileArgs("-trimpath", "-race")).
// Prefer compile-only flags; avoid `-o` / release-style flags here.
func CompileArgs(args ...string) CompileOption {
	return func(c *compileConfig) {
		c.compileArgs = append(c.compileArgs, args...)
	}
}

// VetOption configures the Vet step.
type VetOption func(*vetConfig)

type vetConfig struct {
	vetArgs []string
}

// VetArgs appends additional argv tokens to go vet in [Vet],
// after the step's fixed command prefix.
func VetArgs(args ...string) VetOption {
	return func(c *vetConfig) {
		c.vetArgs = append(c.vetArgs, args...)
	}
}

// TestArgs appends additional argv tokens to go test in [Test] and [CoveredTest],
// after the step's fixed command prefix.
func TestArgs(args ...string) TestOption {
	return func(c *testConfig) {
		c.testArgs = append(c.testArgs, args...)
	}
}
