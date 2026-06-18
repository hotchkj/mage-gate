// Vision: Harness steps consume QualityScopeCommandScope projections without reparsing scope.
package harness_test

import (
	"reflect"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func harnessCrossStepCommandScope(
	t *testing.T,
) (commandScope *gatecheck.QualityScopeCommandScope, sourceFiles []string) {
	t.Helper()
	_, sourceFiles = gatetest.CrossStepParityQualityScopeCommandScope()
	commandScope = testQualityScopeCommandScope(
		gatetest.BDDQualityScopeRows(),
		"testutil",
		[]string{"*_test.go"},
		[]string{"mage", "integration"},
	)
	return commandScope, sourceFiles
}

func assertHarnessCrossStepThresholdFilterParity(
	t *testing.T,
	commandScope *gatecheck.QualityScopeCommandScope,
	wantExSegs, wantPatterns []string,
) {
	t.Helper()
	exSegs, patterns := commandScope.ThresholdPathFilters()
	if !reflect.DeepEqual(exSegs, wantExSegs) {
		t.Fatalf("ThresholdPathFilters exclude segments = %v, want %v", exSegs, wantExSegs)
	}
	if !reflect.DeepEqual(patterns, wantPatterns) {
		t.Fatalf("ThresholdPathFilters patterns = %v, want %v", patterns, wantPatterns)
	}
	covFilter := commandScope.CoverageProfileFilter()
	if !reflect.DeepEqual(covFilter.ExcludeSegments, wantExSegs) {
		t.Fatalf("CoverageProfileFilter excludes = %v, want %v", covFilter.ExcludeSegments, wantExSegs)
	}
	if !reflect.DeepEqual(covFilter.TestFilePatterns, wantPatterns) {
		t.Fatalf("CoverageProfileFilter patterns = %v, want %v", covFilter.TestFilePatterns, wantPatterns)
	}
	if !covFilter.Needed() {
		t.Fatal("CoverageProfileFilter should be active for excludes and test patterns")
	}
}

func assertHarnessCrossStepCommandProjectionsParity(t *testing.T, commandScope *gatecheck.QualityScopeCommandScope) {
	t.Helper()
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	if coverpkg != gatetest.BDDScopedPkgImport {
		t.Fatalf("CoverpkgCSV = %q, want %q", coverpkg, gatetest.BDDScopedPkgImport)
	}
	dirs, err := commandScope.GocycloPkgDirsRootRel()
	if err != nil {
		t.Fatalf("GocycloPkgDirsRootRel: %v", err)
	}
	wantDirs := []string{"zz_bdd_not_default/pkg"}
	if !reflect.DeepEqual(dirs, wantDirs) {
		t.Fatalf("GocycloPkgDirsRootRel = %v, want %v", dirs, wantDirs)
	}
	if commandScope.TagsCSV() != "mage,integration" {
		t.Fatalf("TagsCSV = %q, want mage,integration", commandScope.TagsCSV())
	}
}

func assertHarnessCrossStepMutationParity(
	t *testing.T,
	commandScope *gatecheck.QualityScopeCommandScope,
	sourceFiles []string,
) {
	t.Helper()
	wantExcludeArgv := []string{
		`--exclude-files=^zz_bdd_not_default/pkg/.*_test\.go$`,
		`--exclude-files=^zz_bdd_not_default/testutil(/|$)`,
	}
	scanCommandScope := commandScope.WithSourceInventory(sourceFiles)
	scanCoverpkg, err := scanCommandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("scan CoverpkgCSV: %v", err)
	}
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	if scanCoverpkg != coverpkg {
		t.Fatalf("mutation scan coverpkg %q != covered-test coverpkg %q", scanCoverpkg, coverpkg)
	}
	scanEx, err := scanCommandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatalf("MutationExcludeFileArgv: %v", err)
	}
	if !reflect.DeepEqual(scanEx, wantExcludeArgv) {
		t.Fatalf("mutation excludes = %v, want %v", scanEx, wantExcludeArgv)
	}
}

func assertHarnessCommandScopeMatchesGatecheckFixture(
	t *testing.T,
	commandScope, fixtureCommandScope *gatecheck.QualityScopeCommandScope,
) {
	t.Helper()
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	fixtureCoverpkg, err := fixtureCommandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("fixture CoverpkgCSV: %v", err)
	}
	if coverpkg != fixtureCoverpkg {
		t.Fatalf("harness coverpkg %q != gatecheck fixture %q", coverpkg, fixtureCoverpkg)
	}
	exSegs, patterns := commandScope.ThresholdPathFilters()
	fixtureEx, fixturePatterns := fixtureCommandScope.ThresholdPathFilters()
	if !reflect.DeepEqual(exSegs, fixtureEx) || !reflect.DeepEqual(patterns, fixturePatterns) {
		t.Fatalf("harness command-scope filters (%v, %v) != fixture (%v, %v)", exSegs, patterns, fixtureEx, fixturePatterns)
	}
}

func TestQualityScopeCommandScope_CrossStepThresholdFilterParity(t *testing.T) {
	t.Parallel()
	fixtureCommandScope, _ := gatetest.CrossStepParityQualityScopeCommandScope()
	commandScope, sourceFiles := harnessCrossStepCommandScope(t)
	wantExSegs := []string{"testutil"}
	wantPatterns := []string{"*_test.go"}
	assertHarnessCrossStepThresholdFilterParity(t, commandScope, wantExSegs, wantPatterns)
	assertHarnessCrossStepCommandProjectionsParity(t, commandScope)
	assertHarnessCrossStepMutationParity(t, commandScope, sourceFiles)
	assertHarnessCommandScopeMatchesGatecheckFixture(t, commandScope, &fixtureCommandScope)
}
