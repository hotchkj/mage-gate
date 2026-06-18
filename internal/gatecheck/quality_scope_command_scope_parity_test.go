// Vision: Cross-step parity — all quality-scope consumers share one parsed filter boundary.
package gatecheck

import (
	"reflect"
	"testing"
)

const bddScopedPkgImport = "example.com/mod/zz_bdd_not_default/pkg"

// crossStepParityFixture matches the BDD zz_bdd_not_default scope with testutil excluded and test patterns.
func crossStepParityFixture() (commandScope QualityScopeCommandScope, sourceFiles []string) {
	sourceFiles = []string{
		"zz_bdd_not_default/pkg/pkg.go",
		"zz_bdd_not_default/pkg/pkg_test.go",
		"zz_bdd_not_default/testutil/util.go",
		"zz_bdd_not_default/testutil/util_test.go",
	}
	commandScope = NewQualityScopeCommandScope(
		bddQualityScopeRows(),
		sourceFiles,
		"testutil",
		[]string{"*_test.go"},
		[]string{"mage", "integration"},
	)
	return commandScope, sourceFiles
}

func assertCrossStepThresholdFilterParity(
	t *testing.T,
	commandScope *QualityScopeCommandScope,
	wantExSegs, wantPatterns []string,
) {
	t.Helper()
	exSegs, testPatterns := commandScope.ThresholdPathFilters()
	if !reflect.DeepEqual(exSegs, wantExSegs) {
		t.Fatalf("ThresholdPathFilters exclude segments = %v, want %v", exSegs, wantExSegs)
	}
	if !reflect.DeepEqual(testPatterns, wantPatterns) {
		t.Fatalf("ThresholdPathFilters patterns = %v, want %v", testPatterns, wantPatterns)
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

func assertCrossStepCommandProjectionsParity(t *testing.T, commandScope *QualityScopeCommandScope) {
	t.Helper()
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	if coverpkg != bddScopedPkgImport {
		t.Fatalf("CoverpkgCSV = %q, want %q", coverpkg, bddScopedPkgImport)
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

func assertCrossStepMutationParity(
	t *testing.T,
	commandScope *QualityScopeCommandScope,
	sourceFiles []string,
) {
	t.Helper()
	excludeArgv, err := commandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatalf("MutationExcludeFileArgv: %v", err)
	}
	wantExcludeArgv := []string{
		`--exclude-files=^zz_bdd_not_default/pkg/.*_test\.go$`,
		`--exclude-files=^zz_bdd_not_default/testutil(/|$)`,
	}
	if !reflect.DeepEqual(excludeArgv, wantExcludeArgv) {
		t.Fatalf("MutationExcludeFileArgv = %v, want %v", excludeArgv, wantExcludeArgv)
	}
	coverpkg, err := commandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	scanCommandScope := commandScope.WithSourceInventory(sourceFiles)
	scanCoverpkg, err := scanCommandScope.CoverpkgCSV()
	if err != nil {
		t.Fatalf("scan CoverpkgCSV: %v", err)
	}
	if scanCoverpkg != coverpkg {
		t.Fatalf("scan coverpkg %q != covered-test coverpkg %q", scanCoverpkg, coverpkg)
	}
	scanEx, err := scanCommandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatalf("scan MutationExcludeFileArgv: %v", err)
	}
	if !reflect.DeepEqual(scanEx, excludeArgv) {
		t.Fatalf("scan excludes %v != command-scope excludes %v", scanEx, excludeArgv)
	}
}

func TestQualityScopeCommandScope_CrossStepParsedFilterParity(t *testing.T) {
	t.Parallel()
	commandScope, sourceFiles := crossStepParityFixture()
	wantExSegs := []string{"testutil"}
	wantPatterns := []string{"*_test.go"}
	assertCrossStepThresholdFilterParity(t, &commandScope, wantExSegs, wantPatterns)
	assertCrossStepCommandProjectionsParity(t, &commandScope)
	assertCrossStepMutationParity(t, &commandScope, sourceFiles)
	exSegs, testPatterns := commandScope.ThresholdPathFilters()
	sitesEx, sitesPatterns := commandScope.ThresholdPathFilters()
	if !reflect.DeepEqual(sitesEx, exSegs) || !reflect.DeepEqual(sitesPatterns, testPatterns) {
		t.Fatalf("MutationSites filters drifted: ex=%v pats=%v", sitesEx, sitesPatterns)
	}
}

func TestQualityScopeCommandScope_CrossStepUnscopedParity(t *testing.T) {
	t.Parallel()
	commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", nil, nil)

	exSegs, patterns := commandScope.ThresholdPathFilters()
	if len(exSegs) != 0 || len(patterns) != 0 {
		t.Fatalf("unscoped filters = (%v, %v), want empty", exSegs, patterns)
	}
	if commandScope.CoverageProfileFilter().Needed() {
		t.Fatal("unscoped coverage should not filter profiles")
	}
	excludeArgv, err := commandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatalf("MutationExcludeFileArgv: %v", err)
	}
	if len(excludeArgv) != 0 {
		t.Fatalf("unscoped mutation excludes = %v, want none", excludeArgv)
	}
}
