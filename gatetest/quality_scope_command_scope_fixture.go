// Vision: BDD-shaped quality-scope inventory fixtures shared by gatecheck and harness parity tests.
package gatetest

import "github.com/hotchkj/mage-gate/internal/gatecheck"

// BDDScopedPkgImport is the in-scope import path when zz_bdd_not_default/testutil is excluded.
const BDDScopedPkgImport = "example.com/mod/zz_bdd_not_default/pkg"

// BDDQualityScopeRows returns package inventory rows matching features/*.feature zz_bdd_not_default scope.
func BDDQualityScopeRows() []gatecheck.MutationPackageRow {
	return []gatecheck.MutationPackageRow{
		{
			ImportPath:      "example.com/mod/zz_bdd_not_default/pkg",
			PkgDirRootRel:   "zz_bdd_not_default/pkg",
			GoFileNames:     []string{"pkg.go"},
			TestGoFileNames: []string{"pkg_test.go"},
		},
		{
			ImportPath:      "example.com/mod/zz_bdd_not_default/testutil",
			PkgDirRootRel:   "zz_bdd_not_default/testutil",
			GoFileNames:     []string{"util.go"},
			TestGoFileNames: []string{"util_test.go"},
		},
	}
}

// CrossStepParityQualityScopeCommandScope returns the scoped command scope and source inventory
// used by cross-step parity tests.
func CrossStepParityQualityScopeCommandScope() (commandScope gatecheck.QualityScopeCommandScope, sourceFiles []string) {
	sourceFiles = []string{
		"zz_bdd_not_default/pkg/pkg.go",
		"zz_bdd_not_default/pkg/pkg_test.go",
		"zz_bdd_not_default/testutil/util.go",
		"zz_bdd_not_default/testutil/util_test.go",
	}
	commandScope = gatecheck.NewQualityScopeCommandScope(
		BDDQualityScopeRows(),
		sourceFiles,
		"testutil",
		[]string{"*_test.go"},
		[]string{"mage", "integration"},
	)
	return commandScope, sourceFiles
}
