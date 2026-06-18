//go:build bdd_coverage

// Vision: Single-feature BDD suite selected by `-tags=bdd_coverage` for fast iteration on coverage scenarios.
package features

import "testing"

// featurePaths is scoped to one feature when running: go test -tags=bdd_coverage ./features/...
var featurePaths = []string{
	"coverage.feature",
}

// TestFeaturePathsAreScoped keeps featurePaths aligned with this file's bdd_coverage build tag.
func TestFeaturePathsAreScoped(t *testing.T) {
	t.Helper()
	allowed := map[string]struct{}{
		"coverage.feature": {},
	}

	for _, p := range featurePaths {
		if _, ok := allowed[p]; !ok {
			t.Fatalf("unexpected feature for this tag: %s", p)
		}
	}
}
