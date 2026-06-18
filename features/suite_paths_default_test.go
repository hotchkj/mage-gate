//go:build !bdd_coverage

// Vision: Default `go test ./features` feature list; tagged bdd_coverage suite lives in a separate file.
package features

import "testing"

// featurePaths defines which feature files to include in the default test suite.
// Scoped runs: use -tags=bdd_coverage with suite_paths_bdd_coverage_test.go (single feature).
// The default build constraint must negate every bdd_* tag used by suite_paths_*_test.go files.
var featurePaths = []string{
	"coveredtest.feature",
	"coverage.feature",
	"crap.feature",
	"quality_scope_inventory.feature",
	"deadcode.feature",
	"markdownlint.feature",
	"duration.feature",
	"format.feature",
	"lint.feature",
	"mutationsites.feature",
	"mutation_runner_scan.feature",
	"mutationkills.feature",
	"mutation_runner_scope.feature",
	"mutationcoverage.feature",
	"vet.feature",
	"compile.feature",
	"test_step.feature",
	"config_validation.feature",
	"provenance.feature",
	"step_failure.feature",
}

// TestFeaturePathsAreScoped keeps featurePaths aligned with this file's build tag: a stray filename
// here would run under the wrong suite composition.
func TestFeaturePathsAreScoped(t *testing.T) {
	t.Helper()
	allowed := map[string]struct{}{
		"coveredtest.feature":             {},
		"coverage.feature":                {},
		"crap.feature":                    {},
		"quality_scope_inventory.feature": {},
		"deadcode.feature":                {},
		"markdownlint.feature":            {},
		"duration.feature":                {},
		"format.feature":                  {},
		"lint.feature":                    {},
		"mutationsites.feature":           {},
		"mutation_runner_scan.feature":    {},
		"mutationkills.feature":           {},
		"mutation_runner_scope.feature":   {},
		"mutationcoverage.feature":        {},
		"vet.feature":                     {},
		"compile.feature":                 {},
		"test_step.feature":               {},
		"config_validation.feature":       {},
		"provenance.feature":              {},
		"step_failure.feature":            {},
	}

	for _, p := range featurePaths {
		if _, ok := allowed[p]; !ok {
			t.Fatalf("unexpected feature for this tag: %s", p)
		}
	}
}
