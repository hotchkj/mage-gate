//go:build mage
// +build mage

// Vision: Mutation kill-rate config parsing for mage wiring.
package main

import (
	"testing"
)

func TestParseMutationKillsThreshold(t *testing.T) {
	t.Run("with defined rate", func(t *testing.T) {
		tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
mutation_kills_min_rate = 85
`)
		cfg, err := parseConfig(tomlData)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		rate := cfg.parseMutationKillsThreshold()
		if rate != 85 {
			t.Errorf("expected rate 85, got %d", rate)
		}
	})

	t.Run("with omitted rate defaults to zero", func(t *testing.T) {
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
		rate := cfg.parseMutationKillsThreshold()
		if rate != 0 {
			t.Errorf("expected default rate 0, got %d", rate)
		}
	})
}

func TestParseGremlinsConfigArgsToMutationOptions(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1

[gremlins]
tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
args = ["--timeout=5m"]
`)
	cfg, err := parseConfig(tomlData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Gremlins.Args) != 1 || cfg.Gremlins.Args[0] != "--timeout=5m" {
		t.Fatalf("gremlins args: %v", cfg.Gremlins.Args)
	}
	opts := gremlinsMutationOptions(&cfg)
	if len(opts) != 1 {
		t.Fatalf("expected one mutation option, got %d", len(opts))
	}
}

func TestConfigIntegrationMutationKillsComplete(t *testing.T) {
	t.Run("complete mutation_kills config", func(t *testing.T) {
		tomlData := []byte(`[thresholds]
coverage_min = 90.0
crap_max = 5.0
duration_max = 2.0
mutation_sites_max = 30
mutation_kills_min_rate = 75

[lint]
config = ".golangci.yml"
tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

[quality_scope]
packages = "./..."
exclude = ["testdata", "vendor"]
test_file_patterns = ["*_test.go"]
`)
		cfg, err := parseConfig(tomlData)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}

		if cfg.Thresholds.MutationKillsMinRate == nil || *cfg.Thresholds.MutationKillsMinRate != 75 {
			t.Errorf("expected mutation_kills_min_rate 75, got %v", cfg.Thresholds.MutationKillsMinRate)
		}
		rate := cfg.parseMutationKillsThreshold()
		if rate != 75 {
			t.Errorf("expected rate 75, got %d", rate)
		}
	})
}

func TestConfigMutationSitesAndKillRateThresholdsCoexist(t *testing.T) {
	t.Run("mutation site and kill-rate thresholds coexist", func(t *testing.T) {
		tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 20
mutation_kills_min_rate = 80
`)
		cfg, err := parseConfig(tomlData)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		if cfg.Thresholds.MutationSitesMax == nil || *cfg.Thresholds.MutationSitesMax != 20 {
			t.Errorf("expected mutation_sites_max 20, got %v", cfg.Thresholds.MutationSitesMax)
		}
		if cfg.Thresholds.MutationKillsMinRate == nil || *cfg.Thresholds.MutationKillsMinRate != 80 {
			t.Errorf("expected mutation_kills_min_rate 80, got %v", cfg.Thresholds.MutationKillsMinRate)
		}
	})
}

func TestConfigExplicitMutationSitesMaxPreserved(t *testing.T) {
	tomlData := []byte(`[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 5
mutation_kills_min_rate = 80
`)
	cfg, err := parseConfig(tomlData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Thresholds.MutationSitesMax == nil || *cfg.Thresholds.MutationSitesMax != 5 {
		t.Fatalf("expected mutation_sites_max 5, got %v", cfg.Thresholds.MutationSitesMax)
	}
}
