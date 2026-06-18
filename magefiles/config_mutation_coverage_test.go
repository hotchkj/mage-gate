//go:build mage
// +build mage

// Vision: Parse [thresholds].mutation_coverage_min and map it to gate options.
package main

import (
	"errors"
	"testing"

	qg "github.com/hotchkj/mage-gate/gate"
)

func TestParseMutationCoverageThresholdOmitted(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
`)
	cfg, err := parseConfig(tomlData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Thresholds.MutationCoverageMin != nil {
		t.Fatalf("expected nil MutationCoverageMin when omitted, got %v", cfg.Thresholds.MutationCoverageMin)
	}
	if g := cfg.parseMutationCoverageThreshold(); g != 0 {
		t.Errorf("expected default 0, got %d", g)
	}
	if got := mutationCoverageThreshold(&cfg); got != qg.MinMutationCoverage(0) {
		t.Errorf("mutationCoverageThreshold: got %#v, want MinMutationCoverage(0)", got)
	}
	if mutationCoverageCheckEnabled(&cfg) {
		t.Error("mutationCoverageCheckEnabled: want false when mutation_coverage_min omitted")
	}
}

func TestParseMutationCoverageThresholdDefined(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
mutation_coverage_min = 72
`)
	cfg, err := parseConfig(tomlData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Thresholds.MutationCoverageMin == nil || *cfg.Thresholds.MutationCoverageMin != 72 {
		t.Fatalf("expected mutation_coverage_min 72, got %v", cfg.Thresholds.MutationCoverageMin)
	}
	if g := cfg.parseMutationCoverageThreshold(); g != 72 {
		t.Errorf("expected 72, got %d", g)
	}
	if got := mutationCoverageThreshold(&cfg); got != qg.MinMutationCoverage(72) {
		t.Errorf("mutationCoverageThreshold: got %#v, want MinMutationCoverage(72)", got)
	}
	if !mutationCoverageCheckEnabled(&cfg) {
		t.Error("mutationCoverageCheckEnabled: want true when mutation_coverage_min is positive")
	}
}

func TestParseMutationCoverageThresholdCoexistsWithOtherThresholds(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 20
mutation_kills_min_rate = 80
mutation_coverage_min = 65
`)
	cfg, err := parseConfig(tomlData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Thresholds.MutationSitesMax == nil || *cfg.Thresholds.MutationSitesMax != 20 {
		t.Errorf("mutation_sites_max: got %v", cfg.Thresholds.MutationSitesMax)
	}
	if cfg.Thresholds.MutationKillsMinRate == nil || *cfg.Thresholds.MutationKillsMinRate != 80 {
		t.Errorf("mutation_kills_min_rate: got %v", cfg.Thresholds.MutationKillsMinRate)
	}
	if cfg.Thresholds.MutationCoverageMin == nil || *cfg.Thresholds.MutationCoverageMin != 65 {
		t.Errorf("mutation_coverage_min: got %v", cfg.Thresholds.MutationCoverageMin)
	}
}

// minCoveragePolicyBaseTOML is a minimal valid [thresholds] block for mutation_coverage_min policy tests.
const minCoveragePolicyBaseTOML = `[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
`

// TestMutationCoverageMinOmittedVsExplicitZeroGateMapping documents that the TOML shape differs
// (omitted → nil *int; explicit 0 → non-nil *0) while the mapped gate option is the same:
// MinMutationCoverage(0) disables the check (gate MinMutationCoverage godoc).
func TestMutationCoverageMinOmittedVsExplicitZeroGateMapping(t *testing.T) {
	t.Parallel()
	cfgOmitted, err := parseConfig([]byte(minCoveragePolicyBaseTOML))
	if err != nil {
		t.Fatalf("parseConfig omitted: %v", err)
	}
	if cfgOmitted.Thresholds.MutationCoverageMin != nil {
		t.Fatalf("omitted: want nil MutationCoverageMin, got %v", cfgOmitted.Thresholds.MutationCoverageMin)
	}
	cfgExplicit, err := parseConfig([]byte(minCoveragePolicyBaseTOML + "mutation_coverage_min = 0\n"))
	if err != nil {
		t.Fatalf("parseConfig explicit 0: %v", err)
	}
	if cfgExplicit.Thresholds.MutationCoverageMin == nil {
		t.Fatal("explicit 0: want non-nil *int(0)")
	}
	if *cfgExplicit.Thresholds.MutationCoverageMin != 0 {
		t.Fatalf("explicit 0: want 0, got %d", *cfgExplicit.Thresholds.MutationCoverageMin)
	}
	want := qg.MinMutationCoverage(0)
	if g := mutationCoverageThreshold(&cfgOmitted); g != want {
		t.Errorf("omitted → gate: got %#v, want %#v", g, want)
	}
	if g := mutationCoverageThreshold(&cfgExplicit); g != want {
		t.Errorf("explicit 0 → gate: got %#v, want %#v", g, want)
	}
	if mutationCoverageCheckEnabled(&cfgOmitted) || mutationCoverageCheckEnabled(&cfgExplicit) {
		t.Error("mutationCoverageCheckEnabled: want false for omitted and explicit 0")
	}
}

// A positive mutation_coverage_min maps to a non-zero MinMutationCoverage threshold.
// Omitted and explicit 0 both map to MinMutationCoverage(0) (check disabled; see gate MinMutationCoverage godoc).
func TestMutationCoverageMinEnforceSkipSemantics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		extra           string
		wantGate        qg.MutationCoverageThreshold
		wantParsed      int
		wantGateEnabled bool
	}{
		{
			name:            "omitted",
			extra:           "",
			wantGate:        qg.MinMutationCoverage(0),
			wantParsed:      0,
			wantGateEnabled: false,
		},
		{
			name:            "explicit_zero_skips",
			extra:           "mutation_coverage_min = 0\n",
			wantGate:        qg.MinMutationCoverage(0),
			wantParsed:      0,
			wantGateEnabled: false,
		},
		{
			name:            "positive_enforces_policy",
			extra:           "mutation_coverage_min = 50\n",
			wantGate:        qg.MinMutationCoverage(50),
			wantParsed:      50,
			wantGateEnabled: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			toml := minCoveragePolicyBaseTOML + tc.extra
			cfg, err := parseConfig([]byte(toml))
			if err != nil {
				t.Fatalf("parseConfig: %v", err)
			}
			got := mutationCoverageThreshold(&cfg)
			if got != tc.wantGate {
				t.Fatalf("mutationCoverageThreshold: got %#v, want %#v", got, tc.wantGate)
			}
			if cfg.parseMutationCoverageThreshold() != tc.wantParsed {
				t.Fatalf(
					"parseMutationCoverageThreshold: got %d, want %d",
					cfg.parseMutationCoverageThreshold(),
					tc.wantParsed,
				)
			}
			if got := mutationCoverageCheckEnabled(&cfg); got != tc.wantGateEnabled {
				t.Fatalf("mutationCoverageCheckEnabled: got %v, want %v", got, tc.wantGateEnabled)
			}
		})
	}
}

func TestParseConfigRejectsUnknownMutationCoverageKey(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
mutation_coverage_minimum = 50
`)
	_, err := parseConfig(tomlData)
	if err == nil {
		t.Fatal("expected error for unknown key mutation_coverage_minimum")
	}
	if !errors.Is(err, errUnknownConfigKeys) {
		t.Fatalf("expected errUnknownConfigKeys, got %v", err)
	}
}

// TestParseConfigRejectsLegacyMutationSitesPolicyTypos ensures [thresholds] only accepts the
// supported mutation sites key name; typos must surface as unknown-key errors, not silent drops.
func TestParseConfigRejectsLegacyMutationSitesPolicyTypos(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		badLine string
	}{
		{name: "mutation_threshold", badLine: "mutation_threshold = 50\n"},
		{name: "mutation_sites_threshold", badLine: "mutation_sites_threshold = 50\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			toml := minCoveragePolicyBaseTOML + tc.badLine
			_, err := parseConfig([]byte(toml))
			if err == nil {
				t.Fatal("expected error for unknown legacy key")
			}
			if !errors.Is(err, errUnknownConfigKeys) {
				t.Fatalf("expected errUnknownConfigKeys, got %v", err)
			}
		})
	}
}

func TestParseConfigRejectsMutationCoverageMinOutOfRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		floor string
	}{
		{name: "negative", floor: "mutation_coverage_min = -1\n"},
		{name: "above_100", floor: "mutation_coverage_min = 101\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			toml := minCoveragePolicyBaseTOML + tc.floor
			_, err := parseConfig([]byte(toml))
			if err == nil {
				t.Fatal("expected error for out-of-range mutation_coverage_min")
			}
			if !errors.Is(err, errMutationCoverageMinOutOfRange) {
				t.Fatalf("expected errMutationCoverageMinOutOfRange, got %v", err)
			}
		})
	}
}

func TestParseConfigAcceptsMutationCoverageMin100(t *testing.T) {
	t.Parallel()
	tomlData := minCoveragePolicyBaseTOML + "mutation_coverage_min = 100\n"
	cfg, err := parseConfig([]byte(tomlData))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Thresholds.MutationCoverageMin == nil || *cfg.Thresholds.MutationCoverageMin != 100 {
		t.Fatalf("expected mutation_coverage_min 100, got %v", cfg.Thresholds.MutationCoverageMin)
	}
}

// TestParseConfigRejectsStringMutationCoverageMin enforces the [thresholds].mutation_coverage_min
// schema: integer 0-100, matching gate.MinMutationCoverage. Non-integer TOML values must fail
// at decode (BurntSushi), not be coerced.
func TestParseConfigRejectsStringMutationCoverageMin(t *testing.T) {
	t.Parallel()
	tomlData := minCoveragePolicyBaseTOML + "mutation_coverage_min = \"50\"\n"
	_, err := parseConfig([]byte(tomlData))
	if err == nil {
		t.Fatal("expected error when mutation_coverage_min is a string")
	}
}
