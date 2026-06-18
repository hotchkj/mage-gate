//go:build mage
// +build mage

package main

import "testing"

func TestQualityScopeOptions_UsesQualityScopeExcludeOnly(t *testing.T) {
	t.Parallel()
	cfg := &config{
		QualityScope: qualityScopeConfig{Exclude: []string{"vendor"}},
	}
	opts := qualityScopeOptions(cfg)
	if len(opts) != 1 {
		t.Fatalf("qualityScopeOptions len = %d, want 1", len(opts))
	}
}

func TestQualityScopeOptions_WithTestFilePatterns(t *testing.T) {
	t.Parallel()
	cfg := &config{
		QualityScope: qualityScopeConfig{
			Exclude:          []string{"vendor"},
			TestFilePatterns: []string{"*_test.go"},
		},
	}
	opts := qualityScopeOptions(cfg)
	if len(opts) != 2 {
		t.Fatalf("qualityScopeOptions len = %d, want 2", len(opts))
	}
}

func TestQualityScopeOptions_WithTags(t *testing.T) {
	t.Parallel()
	cfg := &config{
		QualityScope: qualityScopeConfig{
			Tags:             []string{"mage"},
			Exclude:          []string{"vendor"},
			TestFilePatterns: []string{"*_test.go"},
		},
	}
	opts := qualityScopeOptions(cfg)
	if len(opts) != 3 {
		t.Fatalf("qualityScopeOptions len = %d, want 3", len(opts))
	}
}

func TestMutationOptions_UseMutationArgsOnly(t *testing.T) {
	t.Parallel()
	cfg := &config{
		Gremlins: gremlinsConfig{Args: []string{"--timeout=1m"}},
	}
	if got := len(gremlinsMutationOptions(cfg)); got != 1 {
		t.Fatalf("gremlinsMutationOptions len = %d, want 1", got)
	}
}

func TestIsCIReflectsEnvironment(t *testing.T) {
	t.Setenv("CI", "")
	if isCI() {
		t.Fatal("isCI = true, want false when CI is empty")
	}
	t.Setenv("CI", "true")
	if !isCI() {
		t.Fatal("isCI = false, want true when CI is set")
	}
}

func TestToolAndPassOptionsIncludeConfiguredArgs(t *testing.T) {
	t.Parallel()
	cfg := &config{
		Lint: lintConfig{
			CustomGCL:          "custom-gcl",
			CustomLintToolSpec: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1",
			Args:               []string{"--fast"},
		},
		Crap:     crapConfig{Args: []string{"-avg"}},
		Deadcode: deadcodeConfig{Args: []string{"-test"}},
		Markdownlint: markdownlintConfig{
			Args: []string{"--config", ".gomarklint.json"},
		},
		Unittests: unittestsConfig{
			Shuffle: true,
			Args:    []string{"-run", "TestFocused"},
		},
		Integrationtests: integrationtestsConfig{
			Tags:    "integration",
			Shuffle: true,
			Args:    []string{"-run", "TestIntegration"},
		},
	}

	if got := len(lintOptions(cfg)); got != 3 {
		t.Fatalf("lintOptions len = %d, want 3", got)
	}
	if got := len(crapOptions(cfg)); got != 1 {
		t.Fatalf("crapOptions len = %d, want 1", got)
	}
	if got := len(deadcodeOptions(cfg)); got != 1 {
		t.Fatalf("deadcodeOptions len = %d, want 1", got)
	}
	if got := len(markdownlintOpts(cfg)); got != 1 {
		t.Fatalf("markdownlintOpts len = %d, want 1", got)
	}
	if got := len(primaryPassOpts(cfg)); got != 2 {
		t.Fatalf("primaryPassOpts len = %d, want 2", got)
	}
	if got := len(integrationPassOpts(cfg)); got != 3 {
		t.Fatalf("integrationPassOpts len = %d, want 3", got)
	}
}

func TestMarkdownlintEnabledReflectsToolSpec(t *testing.T) {
	t.Parallel()
	if markdownlintEnabled(&config{}) {
		t.Fatal("markdownlintEnabled = true, want false when tool_spec is empty")
	}
	if markdownlintEnabled(&config{Markdownlint: markdownlintConfig{ToolSpec: "   "}}) {
		t.Fatal("markdownlintEnabled = true, want false when tool_spec is whitespace")
	}
	if !markdownlintEnabled(&config{
		Markdownlint: markdownlintConfig{
			ToolSpec: "github.com/shinagawa-web/gomarklint/v3@v3.2.3",
		},
	}) {
		t.Fatal("markdownlintEnabled = false, want true when tool_spec is set")
	}
}

func TestThresholdAndToolValueMappingsAcceptParsedConfig(t *testing.T) {
	t.Parallel()
	cfg := mageFlowConfig()

	_ = lintConfigPath(cfg)
	_ = lintToolSpec(cfg)
	_ = deadcodeToolSpec(cfg)
	_ = markdownlintToolSpec(cfg)
	_ = gocycloToolSpec(cfg)
	_ = gremlinsToolSpec(cfg)
	_ = coverageMin(cfg)
	_ = crapMax(cfg)
	_ = durationMax(cfg)
	_ = mutationSitesMax(cfg)
}
