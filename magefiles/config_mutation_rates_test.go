//go:build mage
// +build mage

// Vision: Mutation kill-rate knobs in gate.toml—defaults, overrides, and invalid combinations for mage wiring.
package main

import (
	"testing"
)

func buildBaseConfigTOML(mutationKillsMinRateLine string) string {
	base := `[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1`
	if mutationKillsMinRateLine != "" {
		base += "\n" + mutationKillsMinRateLine
	}
	return base + "\n"
}

func TestParseConfigMutationKillsMinRate(t *testing.T) {
	t.Run("with valid min rate", func(t *testing.T) {
		tomlData := buildBaseConfigTOML("mutation_kills_min_rate = 80")
		cfg, err := parseConfig([]byte(tomlData))
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		assertMutationKillsMinRateValue(t, &cfg, 80)
	})

	t.Run("omitted min rate is valid", func(t *testing.T) {
		tomlData := buildBaseConfigTOML("")
		cfg, err := parseConfig([]byte(tomlData))
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		assertMutationKillsMinRateNil(t, &cfg)
	})

	t.Run("zero min rate is valid", func(t *testing.T) {
		tomlData := buildBaseConfigTOML("mutation_kills_min_rate = 0")
		cfg, err := parseConfig([]byte(tomlData))
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		assertMutationKillsMinRateValue(t, &cfg, 0)
	})

	t.Run("100.0 min rate is valid", func(t *testing.T) {
		tomlData := buildBaseConfigTOML("mutation_kills_min_rate = 100")
		cfg, err := parseConfig([]byte(tomlData))
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		assertMutationKillsMinRateValue(t, &cfg, 100)
	})
}
