// Vision: CoveredTestOutput.TestRun invariants—nil receiver, empty step ID, empty packages, stored package scope.
package gate

import (
	"errors"
	"testing"
)

func TestCoveredTestOutputTestRunNilReceiver(t *testing.T) {
	t.Parallel()
	var c *CoveredTestOutput
	_, err := c.TestRun()
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v, want ErrMissingValue", err)
	}
}

func TestCoveredTestOutputTestRunEmptyStepID(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, "./...")
	c := CoveredTestOutput{stepID: "", packages: mustNewPackageScope(t, "./..."), qualityScope: scope}
	_, err := c.TestRun()
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v, want ErrMissingValue", err)
	}
}

func TestCoveredTestOutputTestRunCopiesQualityScopeExcludes(t *testing.T) {
	t.Parallel()
	pkg := mustNewPackageScope(t, "./...")
	qs, err := NewQualityScope("./...", Exclude("features", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	c := CoveredTestOutput{stepID: "id", packages: pkg, qualityScope: qs}
	out, err := c.TestRun()
	if err != nil {
		t.Fatalf("TestRun: %v", err)
	}
	if out.stepID != "id" {
		t.Fatalf("stepID = %q, want %q", out.stepID, "id")
	}
}

func TestCoveredTestOutputTestRunReturnsStoredPackages(t *testing.T) {
	t.Parallel()
	pkg := mustNewPackageScope(t, "./cmd/...")
	qs := mustNewQualityScope(t, "./...")
	c := CoveredTestOutput{stepID: "covered-test-1", packages: pkg, qualityScope: qs}
	out, err := c.TestRun()
	if err != nil {
		t.Fatalf("TestRun: %v", err)
	}
	if out.scope.Packages() != "./cmd/..." {
		t.Fatalf("got %q, want ./cmd/...", out.scope.Packages())
	}
}

func TestCoveredTestOutputTestRunEmptyPackages(t *testing.T) {
	t.Parallel()
	c := CoveredTestOutput{stepID: "covered-test-1", qualityScope: mustNewQualityScope(t, "./...")}
	_, err := c.TestRun()
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v, want ErrMissingValue", err)
	}
}
