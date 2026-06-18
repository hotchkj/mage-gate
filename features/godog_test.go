// Invariant: BDD/tests must not construct production wiring (NewProductionRunner, NewProductionFileOps).
// forbidigo enforces that in *_test.go and features/steps (see .golangci.yml); use fake runners and file ops.
// See gate/steps_test.go for CommandRunner injection coverage.
package features

import (
	"testing"

	"github.com/cucumber/godog"
	"github.com/hotchkj/mage-gate/features/internal/steps"
)

func TestFeatures(t *testing.T) {
	t.Helper()
	suite := godog.TestSuite{
		Name: "mage-gate-step-behavior",
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			steps.InitializeGateScenario(ctx)
		},
		Options: &godog.Options{
			Format: "pretty",
			Paths:  featurePaths,
			Strict: true,
		},
	}
	if status := suite.Run(); status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}
