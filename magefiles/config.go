//go:build mage
// +build mage

// Vision: Deserialize gate.toml for mage—plain structs only so magefiles avoid importing the gate library.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/BurntSushi/toml"
)

const defaultQualityScopePackages = "./..."

// errIntegrationPackagesRequired is returned when [integrationtests] sets tags, shuffle, or
// args but omits packages.
var errIntegrationPackagesRequired = errors.New(
	`gate.toml [integrationtests]: "packages" is required when tags, shuffle, or args are set`,
)

// errGateTomlMissing is returned when gate.toml is not present at the configured path.
var errGateTomlMissing = errors.New("gate.toml not found; create gate.toml at repository root")

// errDurationMaxRequired is returned when [thresholds].duration_max is omitted.
var errDurationMaxRequired = errors.New("gate.toml [thresholds]: duration_max is required")

// errCoverageMinRequired is returned when [thresholds].coverage_min is omitted.
var errCoverageMinRequired = errors.New("gate.toml [thresholds]: coverage_min is required")

// errCrapMaxRequired is returned when [thresholds].crap_max is omitted.
var errCrapMaxRequired = errors.New("gate.toml [thresholds]: crap_max is required")

// errMutationSitesMaxRequired is returned when [thresholds].mutation_sites_max is omitted.
var errMutationSitesMaxRequired = errors.New("gate.toml [thresholds]: mutation_sites_max is required")

// errUnknownConfigKeys is returned when gate.toml contains keys not recognised by the current config schema.
var errUnknownConfigKeys = errors.New("gate.toml contains unrecognised keys (removed or misspelled)")

// errMutationCoverageMinOutOfRange is returned when [thresholds].mutation_coverage_min is present
// but not an integer in [0, 100] (same range contract as gate.MinMutationCoverage, where 0 is valid
// and means the check is off when mapped—see that constructor’s godoc).
var errMutationCoverageMinOutOfRange = errors.New(
	`gate.toml [thresholds]: "mutation_coverage_min" must be an integer from 0 to 100`,
)

// errCustomLintToolSpecRequired is returned when [lint].custom_gcl omits its builder pin.
var errCustomLintToolSpecRequired = errors.New(
	`gate.toml [lint]: "custom_lint_tool_spec" is required when "custom_gcl" is set`,
)

// errCustomGCLRequired is returned when [lint].custom_lint_tool_spec has no custom definition path.
var errCustomGCLRequired = errors.New(
	`gate.toml [lint]: "custom_gcl" is required when "custom_lint_tool_spec" is set`,
)

type config struct {
	Thresholds       thresholdConfig        `toml:"thresholds"`
	Lint             lintConfig             `toml:"lint"`
	QualityScope     qualityScopeConfig     `toml:"quality_scope"`
	Deadcode         deadcodeConfig         `toml:"deadcode"`
	Markdownlint     markdownlintConfig     `toml:"markdownlint"`
	Crap             crapConfig             `toml:"crap"`
	Gremlins         gremlinsConfig         `toml:"gremlins"`
	Unittests        unittestsConfig        `toml:"unittests"`
	Integrationtests integrationtestsConfig `toml:"integrationtests"`
}

type thresholdConfig struct {
	CoverageMin          *float64 `toml:"coverage_min"`
	CrapMax              *float64 `toml:"crap_max"`
	DurationMax          *float64 `toml:"duration_max"`
	MutationSitesMax     *int     `toml:"mutation_sites_max"`
	MutationKillsMinRate *int     `toml:"mutation_kills_min_rate"`
	// MutationCoverageMin is optional. Omitted: nil, parsed as 0% → gate MinMutationCoverage(0)
	// (check disabled). Explicit mutation_coverage_min = 0: non-nil *0, same gate value
	// MinMutationCoverage(0) (check disabled). Positive values are the minimum required when a
	// MutationCoverage step runs.
	MutationCoverageMin *int `toml:"mutation_coverage_min"`
}

type lintConfig struct {
	Config             string   `toml:"config"`
	CustomGCL          string   `toml:"custom_gcl"`
	CustomLintToolSpec string   `toml:"custom_lint_tool_spec"`
	ToolSpec           string   `toml:"tool_spec"`
	Args               []string `toml:"args"`
}

type qualityScopeConfig struct {
	Packages         string   `toml:"packages"`
	Tags             []string `toml:"tags"`
	Exclude          []string `toml:"exclude"`
	TestFilePatterns []string `toml:"test_file_patterns"`
}

type deadcodeConfig struct {
	Args     []string `toml:"args"`
	ToolSpec string   `toml:"tool_spec"`
}

type markdownlintConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

type crapConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

// gremlinsConfig holds the shared gremlins module pin for MutationSites and MutationKills.
type gremlinsConfig struct {
	ToolSpec string   `toml:"tool_spec"`
	Args     []string `toml:"args"`
}

type unittestsConfig struct {
	Shuffle bool     `toml:"shuffle"`
	Args    []string `toml:"args"`
}

// integrationtestsConfig is an optional second `go test` in the same gate (see magefiles).
type integrationtestsConfig struct {
	Packages string   `toml:"packages"`
	Tags     string   `toml:"tags"`
	Shuffle  bool     `toml:"shuffle"`
	Args     []string `toml:"args"`
}

// validateIntegrationConfig rejects a partially filled [integrationtests] (tags/shuffle/args
// without packages). An entirely empty block is valid and means "skip integration tests".
func (cfg *config) validateIntegrationConfig() error {
	pkgs := strings.TrimSpace(cfg.Integrationtests.Packages)
	sec := cfg.Integrationtests
	hasOpts := strings.TrimSpace(sec.Tags) != "" || sec.Shuffle || len(sec.Args) > 0
	if pkgs == "" && hasOpts {
		return errIntegrationPackagesRequired
	}
	return nil
}

// configReader reads file contents for loadConfig; production uses os.ReadFile.
type configReader func(string) ([]byte, error)

func loadConfig(path string, read configReader) (config, error) {
	// #nosec G304 -- path is supplied by the caller; production uses constant policyPath via os.ReadFile.
	data, err := read(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return config{}, errGateTomlMissing
		}
		return config{}, fmt.Errorf("read config: %w", err)
	}
	return parseConfig(data)
}

func parseConfig(data []byte) (config, error) {
	var parsed config
	md, err := toml.Decode(string(data), &parsed)
	if err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return config{}, fmt.Errorf("%w: %s", errUnknownConfigKeys, strings.Join(keys, ", "))
	}
	if err := validateMandatoryDurationAndMutation(&md, &parsed); err != nil {
		return config{}, err
	}
	if err := validateOptionalMutationCoverageMin(&md, &parsed); err != nil {
		return config{}, err
	}
	if err := validateLintCustomConfig(&parsed); err != nil {
		return config{}, err
	}
	return parsed, nil
}

func requireDefinedFloat64Threshold(
	md *toml.MetaData,
	field string,
	value *float64,
	errMissing error,
) error {
	if !md.IsDefined("thresholds", field) {
		return errMissing
	}
	if value == nil {
		return errMissing
	}
	return nil
}

func requireDefinedIntThreshold(
	md *toml.MetaData,
	field string,
	value *int,
	errMissing error,
) error {
	if !md.IsDefined("thresholds", field) {
		return errMissing
	}
	if value == nil {
		return errMissing
	}
	return nil
}

// validateMandatoryDurationAndMutation enforces that duration and mutationsites steps have
// explicit policy in gate.toml (no implicit zero thresholds or omitted tables).
// Note: mutationkills is on-demand (not part of Gate()) so [mutation_kills] table is optional.
func validateMandatoryDurationAndMutation(md *toml.MetaData, cfg *config) error {
	if err := requireDefinedFloat64Threshold(
		md, "coverage_min", cfg.Thresholds.CoverageMin, errCoverageMinRequired,
	); err != nil {
		return err
	}
	if err := requireDefinedFloat64Threshold(md, "crap_max", cfg.Thresholds.CrapMax, errCrapMaxRequired); err != nil {
		return err
	}
	if err := requireDefinedFloat64Threshold(
		md, "duration_max", cfg.Thresholds.DurationMax, errDurationMaxRequired,
	); err != nil {
		return err
	}
	if err := requireDefinedIntThreshold(
		md, "mutation_sites_max", cfg.Thresholds.MutationSitesMax, errMutationSitesMaxRequired,
	); err != nil {
		return err
	}
	// mutationkills is optional (on-demand, not part of Gate)
	return nil
}

// validateOptionalMutationCoverageMin enforces 0-100 for mutation_coverage_min when the key
// is set (including 0, which is valid and opts out the same as omitting the key at the gate
// option layer). Omitted key means MutationCoverageMin stays nil; parse still yields 0%.
func validateOptionalMutationCoverageMin(md *toml.MetaData, cfg *config) error {
	if !md.IsDefined("thresholds", "mutation_coverage_min") {
		return nil
	}
	if cfg.Thresholds.MutationCoverageMin == nil {
		return errMutationCoverageMinOutOfRange
	}
	v := *cfg.Thresholds.MutationCoverageMin
	if v < 0 || v > 100 {
		return fmt.Errorf("%w: got %d", errMutationCoverageMinOutOfRange, v)
	}
	return nil
}

func validateLintCustomConfig(cfg *config) error {
	if cfg.Lint.CustomGCL != "" && cfg.Lint.CustomLintToolSpec == "" {
		return errCustomLintToolSpecRequired
	}
	if cfg.Lint.CustomLintToolSpec != "" && cfg.Lint.CustomGCL == "" {
		return errCustomGCLRequired
	}
	return nil
}

func (cfg *config) packages() string {
	if cfg.QualityScope.Packages == "" {
		return defaultQualityScopePackages
	}
	return cfg.QualityScope.Packages
}

// parseMutationKillsThreshold extracts the min-rate percentage threshold from config.
// Returns zero if mutation_kills_min_rate is not defined (optional).
func (cfg *config) parseMutationKillsThreshold() int {
	if cfg.Thresholds.MutationKillsMinRate != nil {
		return *cfg.Thresholds.MutationKillsMinRate
	}
	return 0
}

// parseMutationCoverageThreshold returns the min mutation coverage percentage. If the key is
// omitted, returns 0 (nil pointer). If the key is present with 0, returns 0 (non-nil *0). Both
// cases map to qg.MinMutationCoverage(0), which disables the threshold check per gate docs.
func (cfg *config) parseMutationCoverageThreshold() int {
	if cfg.Thresholds.MutationCoverageMin != nil {
		return *cfg.Thresholds.MutationCoverageMin
	}
	return 0
}
