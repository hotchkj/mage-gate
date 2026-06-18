//go:build mage
// +build mage

// Vision: Translate parsed TOML into gate functional options plus human-readable progress for mage targets.
package main

import (
	"fmt"
	"os"
	"strings"

	qg "github.com/hotchkj/mage-gate/gate"
)

// isCI reports whether the current process is running under a CI environment.
// Checks the standard CI environment variable set by GitHub Actions and most CI systems.
func isCI() bool {
	return os.Getenv("CI") != ""
}

func qualityScopeOptions(cfg *config) []qg.QualityScopeOption {
	var opts []qg.QualityScopeOption
	if len(cfg.QualityScope.Tags) > 0 {
		opts = append(opts, qg.Tags(cfg.QualityScope.Tags...))
	}
	if len(cfg.QualityScope.Exclude) > 0 {
		opts = append(opts, qg.Exclude(cfg.QualityScope.Exclude...))
	}
	if len(cfg.QualityScope.TestFilePatterns) > 0 {
		opts = append(opts, qg.TestFilePatterns(cfg.QualityScope.TestFilePatterns...))
	}
	return opts
}

func lintOptions(cfg *config) []qg.LintOption {
	var opts []qg.LintOption
	if cfg.Lint.CustomGCL != "" {
		opts = append(opts, qg.CustomGCL(cfg.Lint.CustomGCL))
	}
	if cfg.Lint.CustomLintToolSpec != "" {
		opts = append(opts, qg.CustomLintToolSpec(cfg.Lint.CustomLintToolSpec))
	}
	if len(cfg.Lint.Args) > 0 {
		opts = append(opts, qg.LintArgs(cfg.Lint.Args...))
	}
	return opts
}

func crapOptions(cfg *config) []qg.CrapOption {
	var opts []qg.CrapOption
	if len(cfg.Crap.Args) > 0 {
		opts = append(opts, qg.CrapArgs(cfg.Crap.Args...))
	}
	return opts
}

func deadcodeOptions(cfg *config) []qg.DeadcodeOption {
	var opts []qg.DeadcodeOption
	if len(cfg.Deadcode.Args) > 0 {
		opts = append(opts, qg.DeadcodeArgs(cfg.Deadcode.Args...))
	}
	return opts
}

func markdownlintEnabled(cfg *config) bool {
	return strings.TrimSpace(cfg.Markdownlint.ToolSpec) != ""
}

func markdownlintOpts(cfg *config) []qg.MarkdownLintOption {
	var opts []qg.MarkdownLintOption
	if len(cfg.Markdownlint.Args) > 0 {
		opts = append(opts, qg.MarkdownLintArgs(cfg.Markdownlint.Args...))
	}
	return opts
}

// primaryPassOpts maps [unittests] to gate.Test options for the coverage-bearing pass.
func primaryPassOpts(cfg *config) []qg.TestOption {
	var opts []qg.TestOption
	if cfg.Unittests.Shuffle {
		opts = append(opts, qg.TestArgs("-shuffle=on"))
	}
	if len(cfg.Unittests.Args) > 0 {
		opts = append(opts, qg.TestArgs(cfg.Unittests.Args...))
	}
	return opts
}

// integrationPassOpts maps [integrationtests] to gate.Test options (integration uses [gate.Test] only).
func integrationPassOpts(cfg *config) []qg.TestOption {
	var opts []qg.TestOption
	if t := strings.TrimSpace(cfg.Integrationtests.Tags); t != "" {
		opts = append(opts, qg.TestArgs("-tags="+t))
	}
	if cfg.Integrationtests.Shuffle {
		opts = append(opts, qg.TestArgs("-shuffle=on"))
	}
	if len(cfg.Integrationtests.Args) > 0 {
		opts = append(opts, qg.TestArgs(cfg.Integrationtests.Args...))
	}
	return opts
}

// gremlinsMutationOptions maps [gremlins] args to [qg.MutationOption] for scan and kill mage targets.
func gremlinsMutationOptions(cfg *config) []qg.MutationOption {
	var opts []qg.MutationOption
	if len(cfg.Gremlins.Args) > 0 {
		opts = append(opts, qg.MutationArgs(cfg.Gremlins.Args...))
	}
	return opts
}

func mutationKillsThreshold(cfg *config) qg.MinKillRateThreshold {
	// mutation_kills_min_rate is optional; default is 0 (check disabled).
	rate := cfg.parseMutationKillsThreshold()
	return qg.MinKillRate(rate)
}

// mutationCoverageThreshold maps gate.toml [thresholds].mutation_coverage_min to qg.MinMutationCoverage.
// Omitted key and explicit 0 are distinct in config (nil vs *0) but both yield MinMutationCoverage(0),
// which disables the check—the same semantics as passing 0 to MinMutationCoverage in Go (see gate godoc).
// Used by the main Gate() target when applying [MutationCoverage] after [MutationSites].
func mutationCoverageThreshold(cfg *config) qg.MutationCoverageThreshold {
	pct := cfg.parseMutationCoverageThreshold()
	return qg.MinMutationCoverage(pct)
}

// mutationCoverageCheckEnabled is true when gate.toml sets a positive mutation_coverage_min, so the
// Gate() mutation phase should run [MutationCoverage] after [MutationSites]. Omitted or zero disables
// the step entirely (no MutationCoverage call).
func mutationCoverageCheckEnabled(cfg *config) bool {
	return cfg.parseMutationCoverageThreshold() > 0
}

var (
	readPolicyFile   = os.ReadFile
	newArtifactStore = qg.NewArtifactStore
	newFileOps       = qg.NewProductionFileOps
	newResolver      = qg.NewProductionToolResolver
)

var newRunner = func() (qg.CommandRunner, error) {
	mode := qg.OutputModeAgent
	if isCI() {
		mode = qg.OutputModeVerbose
	}
	return qg.NewDisplayRunner(qg.NewProductionRunner(), mode, os.Stdout, os.Stderr)
}

func lintConfigPath(cfg *config) qg.LintConfigValue {
	return qg.LintConfig(cfg.Lint.Config)
}

func lintToolSpec(cfg *config) qg.LintToolValue {
	return qg.LintToolSpec(cfg.Lint.ToolSpec)
}

func lintToolchain(cfg *config) (qg.LintToolchain, error) {
	return qg.NewLintToolchain(
		lintConfigPath(cfg),
		lintToolSpec(cfg),
		lintOptions(cfg)...,
	)
}

func deadcodeToolSpec(cfg *config) qg.DeadcodeToolValue {
	return qg.DeadcodeToolSpec(cfg.Deadcode.ToolSpec)
}

func markdownlintToolSpec(cfg *config) qg.MarkdownLintToolValue {
	return qg.MarkdownLintToolSpec(cfg.Markdownlint.ToolSpec)
}

func gocycloToolSpec(cfg *config) qg.GocycloToolValue {
	return qg.GocycloToolSpec(cfg.Crap.ToolSpec)
}

func gremlinsToolSpec(cfg *config) qg.GremlinsToolValue {
	return qg.GremlinsToolSpec(cfg.Gremlins.ToolSpec)
}

func coverageMin(cfg *config) qg.CoverageThreshold {
	return qg.MinPercent(*cfg.Thresholds.CoverageMin)
}

func crapMax(cfg *config) qg.CrapThreshold {
	return qg.MaxScore(*cfg.Thresholds.CrapMax)
}

func durationMax(cfg *config) qg.DurationThreshold {
	// Invariant: parseConfig requires [thresholds].duration_max.
	return qg.MaxSeconds(*cfg.Thresholds.DurationMax)
}

func mutationSitesMax(cfg *config) qg.MutationSitesThreshold {
	// Invariant: parseConfig requires [thresholds].mutation_sites_max.
	return qg.MaxSites(*cfg.Thresholds.MutationSitesMax)
}

func printProgress(message string) {
	fmt.Println(message)
}
