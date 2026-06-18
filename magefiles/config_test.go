//go:build mage
// +build mage

// Vision: gate.toml parsing in mage targets—tables map to gate options without importing the gate package.
//
//nolint:revive // File contains comprehensive tests; length justified by coverage requirements
package main

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
)

var errTestDiskFailure = errors.New("disk failure")

const (
	testdata                = "testdata"
	baseMutationKillsConfig = `[thresholds]
coverage_min = 88
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
`
)

// minDurationMutationSitesPolicy is the smallest fragment that satisfies
// validateMandatoryDurationAndMutation; append scenario-specific TOML after it.
const minDurationMutationSitesPolicy = `[thresholds]
coverage_min = 90
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1

`

func TestLoadConfigMissingFile(t *testing.T) {
	fakeRead := func(_ string) ([]byte, error) {
		return nil, fs.ErrNotExist
	}
	_, err := loadConfig("any-path.toml", fakeRead)
	if !errors.Is(err, errGateTomlMissing) {
		t.Fatalf("expected errGateTomlMissing, got %v", err)
	}
}

func TestLoadConfigReadError(t *testing.T) {
	fakeRead := func(_ string) ([]byte, error) {
		return nil, errTestDiskFailure
	}
	_, err := loadConfig("any-path.toml", fakeRead)
	if err == nil {
		t.Fatal("expected error for read failure")
	}
	if !errors.Is(err, errTestDiskFailure) {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}

func TestParseConfigValidTOML(t *testing.T) {
	content := []byte(`[thresholds]
coverage_min = 95.0
crap_max = 5.0
duration_max = 2.0
mutation_sites_max = 30

[lint]
config = ".golangci.yml"
custom_gcl = ".custom-gcl.yml"
custom_lint_tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

[deadcode]
args = ["-tags=my_tags"]
tool_spec = "golang.org/x/tools/cmd/deadcode@v0.31.0"

[markdownlint]
tool_spec = "github.com/shinagawa-web/gomarklint/v3@v3.2.3"
args = ["--config", ".gomarklint.json"]

[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

[gremlins]
tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

[quality_scope]
packages = "./..."
tags = ["mage"]
exclude = ["vendor", "testdata"]
test_file_patterns = ["*_test.go"]
`)
	cfg, err := parseConfig(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertThresholds(t, &cfg)
	assertLintConfig(t, &cfg)
	assertQualityScopeConfig(t, &cfg)
	assertDeadcodeConfig(t, &cfg)
	assertMarkdownlintConfig(t, &cfg)
	assertCrapAndGremlinsToolSpecs(t, &cfg)
}

func assertThresholds(t *testing.T, cfg *config) {
	t.Helper()
	if cfg.Thresholds.CoverageMin == nil || *cfg.Thresholds.CoverageMin != 95.0 {
		t.Errorf("expected CoverageMin=95.0, got %v", cfg.Thresholds.CoverageMin)
	}
	if cfg.Thresholds.CrapMax == nil || *cfg.Thresholds.CrapMax != 5.0 {
		t.Errorf("expected CrapMax=5.0, got %v", cfg.Thresholds.CrapMax)
	}
	if cfg.Thresholds.DurationMax == nil || *cfg.Thresholds.DurationMax != 2.0 {
		t.Errorf("expected DurationMax=2.0, got %v", cfg.Thresholds.DurationMax)
	}
	if cfg.Thresholds.MutationSitesMax == nil || *cfg.Thresholds.MutationSitesMax != 30 {
		t.Errorf("expected MutationSitesMax=30, got %v", cfg.Thresholds.MutationSitesMax)
	}
}

func assertLintConfig(t *testing.T, cfg *config) {
	t.Helper()
	if cfg.Lint.Config != ".golangci.yml" {
		t.Errorf("expected Lint Config=.golangci.yml, got %s", cfg.Lint.Config)
	}
	if cfg.Lint.CustomGCL != ".custom-gcl.yml" {
		t.Errorf("expected CustomGCL=.custom-gcl.yml, got %s", cfg.Lint.CustomGCL)
	}
	if cfg.Lint.CustomLintToolSpec != "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1" {
		t.Errorf(
			"expected CustomLintToolSpec=github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1, got %s",
			cfg.Lint.CustomLintToolSpec,
		)
	}
	if cfg.Lint.ToolSpec != "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1" {
		t.Errorf(
			"expected ToolSpec=github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1, got %s",
			cfg.Lint.ToolSpec,
		)
	}
}

func assertQualityScopeConfig(t *testing.T, cfg *config) {
	t.Helper()
	if cfg.QualityScope.Packages != "./..." {
		t.Errorf("expected Packages=%s, got %s", "./...", cfg.QualityScope.Packages)
	}
	if !slices.Equal(cfg.QualityScope.Tags, []string{"mage"}) {
		t.Errorf("expected tags [mage], got %v", cfg.QualityScope.Tags)
	}
	if len(cfg.QualityScope.Exclude) != 2 {
		t.Fatalf("expected 2 excludes, got %d", len(cfg.QualityScope.Exclude))
	}
	if !slices.Equal(cfg.QualityScope.Exclude, []string{"vendor", testdata}) {
		t.Errorf("expected excludes [vendor, testdata], got %v", cfg.QualityScope.Exclude)
	}
	if len(cfg.QualityScope.TestFilePatterns) != 1 || cfg.QualityScope.TestFilePatterns[0] != "*_test.go" {
		t.Errorf("expected test_file_patterns [*_test.go], got %v", cfg.QualityScope.TestFilePatterns)
	}
}

func assertDeadcodeConfig(t *testing.T, cfg *config) {
	t.Helper()
	if len(cfg.Deadcode.Args) != 1 || cfg.Deadcode.Args[0] != "-tags=my_tags" {
		t.Errorf("expected Args [-tags=my_tags], got %v", cfg.Deadcode.Args)
	}
	if cfg.Deadcode.ToolSpec != "golang.org/x/tools/cmd/deadcode@v0.31.0" {
		t.Errorf("expected ToolSpec=golang.org/x/tools/cmd/deadcode@v0.31.0, got %s", cfg.Deadcode.ToolSpec)
	}
}

func assertMarkdownlintConfig(t *testing.T, cfg *config) {
	t.Helper()
	wantSpec := "github.com/shinagawa-web/gomarklint/v3@v3.2.3"
	if cfg.Markdownlint.ToolSpec != wantSpec {
		t.Errorf("expected Markdownlint ToolSpec=%q, got %q", wantSpec, cfg.Markdownlint.ToolSpec)
	}
	wantArgs := []string{"--config", ".gomarklint.json"}
	if !slices.Equal(cfg.Markdownlint.Args, wantArgs) {
		t.Errorf("expected Markdownlint Args=%v, got %v", wantArgs, cfg.Markdownlint.Args)
	}
}

func assertCrapAndGremlinsToolSpecs(t *testing.T, cfg *config) {
	t.Helper()
	wantGocyclo := "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"
	if cfg.Crap.ToolSpec != wantGocyclo {
		t.Errorf("expected Crap ToolSpec=%q, got %q", wantGocyclo, cfg.Crap.ToolSpec)
	}
	wantGremlins := "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
	if cfg.Gremlins.ToolSpec != wantGremlins {
		t.Errorf("expected Gremlins ToolSpec=%q, got %q", wantGremlins, cfg.Gremlins.ToolSpec)
	}
}

func TestParseConfigInvalidTOML(t *testing.T) {
	_, err := parseConfig([]byte("not valid toml {{{"))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestParseConfigCoverageMinExplicitZero(t *testing.T) {
	content := []byte(`[thresholds]
coverage_min = 0
crap_max = 8
duration_max = 0.1
mutation_sites_max = 1
`)
	cfg, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Thresholds.CoverageMin == nil {
		t.Fatal("expected CoverageMin for explicit zero")
	}
	if *cfg.Thresholds.CoverageMin != 0 {
		t.Fatalf("got %f", *cfg.Thresholds.CoverageMin)
	}
}

func TestParseConfigCoverageMinOmitted(t *testing.T) {
	content := []byte(`[thresholds]
crap_max = 8.0
duration_max = 1.0
mutation_sites_max = 1
`)
	_, err := parseConfig(content)
	if !errors.Is(err, errCoverageMinRequired) {
		t.Fatalf("expected errCoverageMinRequired, got %v", err)
	}
}

func TestParseConfigCrapMaxOmitted(t *testing.T) {
	content := []byte(`[thresholds]
coverage_min = 88.0
duration_max = 1.0
mutation_sites_max = 1
`)
	_, err := parseConfig(content)
	if !errors.Is(err, errCrapMaxRequired) {
		t.Fatalf("expected errCrapMaxRequired, got %v", err)
	}
}

func TestParseConfigRejectsIncompleteCustomLintPair(t *testing.T) {
	t.Parallel()
	t.Run("custom_gcl without custom_lint_tool_spec", func(t *testing.T) {
		content := []byte(minDurationMutationSitesPolicy + `[lint]
custom_gcl = ".custom-gcl.yml"
`)
		_, err := parseConfig(content)
		if !errors.Is(err, errCustomLintToolSpecRequired) {
			t.Fatalf("expected errCustomLintToolSpecRequired, got %v", err)
		}
	})

	t.Run("custom_lint_tool_spec without custom_gcl", func(t *testing.T) {
		content := []byte(minDurationMutationSitesPolicy + `[lint]
custom_lint_tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
`)
		_, err := parseConfig(content)
		if !errors.Is(err, errCustomGCLRequired) {
			t.Fatalf("expected errCustomGCLRequired, got %v", err)
		}
	})
}

func assertMutationKillsMinRateValue(t *testing.T, cfg *config, expectedRate int) {
	t.Helper()
	if cfg.Thresholds.MutationKillsMinRate == nil || *cfg.Thresholds.MutationKillsMinRate != expectedRate {
		t.Fatalf("expected mutation_kills_min_rate %d, got %v", expectedRate, cfg.Thresholds.MutationKillsMinRate)
	}
}

func assertMutationKillsMinRateNil(t *testing.T, cfg *config) {
	t.Helper()
	if cfg.Thresholds.MutationKillsMinRate != nil {
		t.Fatalf("expected nil for omitted mutation_kills_min_rate, got %v", cfg.Thresholds.MutationKillsMinRate)
	}
}

func TestParseConfigUnittestsSection(t *testing.T) {
	t.Run("shuffle", func(t *testing.T) {
		content := []byte(minDurationMutationSitesPolicy + "[unittests]\nshuffle = true\n")
		cfg, err := parseConfig(content)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		if !cfg.Unittests.Shuffle {
			t.Error("expected Shuffle=true")
		}
	})

	t.Run("args", func(t *testing.T) {
		content := []byte(minDurationMutationSitesPolicy + "[unittests]\nargs = [\"-v\", \"-run\", \"TestSpecific\"]\n")
		cfg, err := parseConfig(content)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		if len(cfg.Unittests.Args) != 3 {
			t.Fatalf("expected 3 args, got %d", len(cfg.Unittests.Args))
		}
	})

	t.Run("shuffle and args", func(t *testing.T) {
		content := []byte(minDurationMutationSitesPolicy + "[unittests]\nshuffle = true\nargs = [\"-v\"]\n")
		cfg, err := parseConfig(content)
		if err != nil {
			t.Fatalf("parseConfig: %v", err)
		}
		if !cfg.Unittests.Shuffle || len(cfg.Unittests.Args) != 1 || cfg.Unittests.Args[0] != "-v" {
			t.Fatalf("expected shuffle and one arg, got shuffle=%v args=%v", cfg.Unittests.Shuffle, cfg.Unittests.Args)
		}
	})
}

func TestParseConfigIntegrationSection(t *testing.T) {
	content := []byte(minDurationMutationSitesPolicy + `
[unittests]
shuffle = true

[integrationtests]
packages = "./integration/..."
tags = "integration"
shuffle = false
args = ["-count=1"]
`)
	cfg, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Unittests.Shuffle {
		t.Error("expected primary unittests shuffle=true")
	}
	if cfg.Integrationtests.Packages != "./integration/..." {
		t.Errorf("Integrationtests.Packages=%q", cfg.Integrationtests.Packages)
	}
	if cfg.Integrationtests.Tags != "integration" {
		t.Errorf("Integrationtests.Tags=%q", cfg.Integrationtests.Tags)
	}
	if cfg.Integrationtests.Shuffle {
		t.Error("expected integrationtests shuffle=false independent of [unittests]")
	}
	if len(cfg.Integrationtests.Args) != 1 || cfg.Integrationtests.Args[0] != "-count=1" {
		t.Errorf("Integrationtests.Args=%v", cfg.Integrationtests.Args)
	}
}

func assertParseConfigError(t *testing.T, toml string, want error) {
	t.Helper()
	_, err := parseConfig([]byte(toml))
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestParseConfigRejectsMissingThresholdPolicy(t *testing.T) {
	t.Run("omitted coverage_min", func(t *testing.T) {
		assertParseConfigError(t, testTOMLMissingCoverageMin, errCoverageMinRequired)
	})
	t.Run("omitted crap_max", func(t *testing.T) {
		assertParseConfigError(t, testTOMLMissingCrapMax, errCrapMaxRequired)
	})
	t.Run("omitted duration_max", func(t *testing.T) {
		assertParseConfigError(t, testTOMLMissingDurationMax, errDurationMaxRequired)
	})
	t.Run("omitted mutation_sites_max", func(t *testing.T) {
		assertParseConfigError(t, testTOMLMissingMutationSitesMax, errMutationSitesMaxRequired)
	})
}

func TestParseConfigAcceptsNewMutationSitesKeys(t *testing.T) {
	t.Run("new [mutation_sites] table and mutation_sites_max", func(t *testing.T) {
		toml := `[thresholds]
coverage_min = 88.0
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
`
		_, err := parseConfig([]byte(toml))
		if err != nil {
			t.Fatalf("expected no error with new keys, got %v", err)
		}
	})
}

const (
	testTOMLMissingCoverageMin = `
[thresholds]
crap_max = 8
duration_max = 1.0
mutation_sites_max = 1
`
	testTOMLMissingCrapMax = `
[thresholds]
coverage_min = 88.0
duration_max = 1.0
mutation_sites_max = 1
`
	testTOMLMissingDurationMax = `
[thresholds]
coverage_min = 88.0
crap_max = 8
mutation_sites_max = 1
`
	testTOMLMissingMutationSitesMax = `
[thresholds]
coverage_min = 88.0
crap_max = 8
duration_max = 1.0
`
)

func TestValidateIntegrationSection(t *testing.T) {
	t.Run("empty integrationtests is valid", func(t *testing.T) {
		var cfg config
		if err := cfg.validateIntegrationConfig(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("tags without packages", func(t *testing.T) {
		cfg := config{Integrationtests: integrationtestsConfig{Tags: "integration"}}
		err := cfg.validateIntegrationConfig()
		if !errors.Is(err, errIntegrationPackagesRequired) {
			t.Fatalf("expected errIntegrationPackagesRequired, got %v", err)
		}
	})
	t.Run("shuffle without packages", func(t *testing.T) {
		cfg := config{Integrationtests: integrationtestsConfig{Shuffle: true}}
		err := cfg.validateIntegrationConfig()
		if !errors.Is(err, errIntegrationPackagesRequired) {
			t.Fatalf("expected errIntegrationPackagesRequired, got %v", err)
		}
	})
	t.Run("args without packages", func(t *testing.T) {
		cfg := config{Integrationtests: integrationtestsConfig{Args: []string{"-v"}}}
		err := cfg.validateIntegrationConfig()
		if !errors.Is(err, errIntegrationPackagesRequired) {
			t.Fatalf("expected errIntegrationPackagesRequired, got %v", err)
		}
	})
	t.Run("packages with tags ok", func(t *testing.T) {
		cfg := config{Integrationtests: integrationtestsConfig{
			Packages: "./integration/...",
			Tags:     "integration",
		}}
		if err := cfg.validateIntegrationConfig(); err != nil {
			t.Fatalf("unexpected %v", err)
		}
	})
}

func TestConfigPackages(t *testing.T) {
	t.Run("defaults to ./...", func(t *testing.T) {
		var cfg config
		if cfg.packages() != "./..." {
			t.Errorf("expected %s, got %s", "./...", cfg.packages())
		}
	})

	t.Run("returns configured value", func(t *testing.T) {
		cfg := config{QualityScope: qualityScopeConfig{Packages: "./internal/..."}}
		if cfg.packages() != "./internal/..." {
			t.Errorf("expected ./internal/..., got %s", cfg.packages())
		}
	})
}
