//go:build mage
// +build mage

// Vision: Parsed gate policy must keep the same [quality_scope] coverage boundary as the real
// gate.toml (BDD + harness + integration + testdata excluded from the scored set).
package main

import (
	"slices"
	"testing"
)

// minimalTomlWithRepoQualityScope is valid policy matching this repository’s gate.toml
// [quality_scope] exclude list; keep in sync when changing gate.toml.
const minimalTomlWithRepoQualityScope = `[thresholds]
coverage_min = 90.0
crap_max = 8.0
duration_max = 1.5
mutation_sites_max = 50
mutation_coverage_min = 80

[lint]
config = ".golangci.yml"
tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

[quality_scope]
packages = "./..."
tags = ["mage"]
exclude = ["cmdtest", "features", "gatetest", "integration", "testdata"]
test_file_patterns = ["*_test.go"]

[deadcode]
tool_spec = "golang.org/x/tools/cmd/deadcode@v0.31.0"

[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

[gremlins]
tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
`

func TestParsedPolicy_QualityScopeExcludesBDDAndHarness(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]byte(minimalTomlWithRepoQualityScope))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	want := []string{"cmdtest", "features", "gatetest", "integration", "testdata"}
	if !slices.Equal(cfg.QualityScope.Exclude, want) {
		t.Fatalf("quality_scope.exclude\ngot  %#v\nwant %#v", cfg.QualityScope.Exclude, want)
	}
	if !slices.Equal(cfg.QualityScope.Tags, []string{"mage"}) {
		t.Fatalf("quality_scope.tags got %#v want [mage]", cfg.QualityScope.Tags)
	}
	opts := qualityScopeOptions(&cfg)
	if len(opts) < 1 {
		t.Fatal("expected QualityScope options from config with excludes and test file patterns")
	}
}
