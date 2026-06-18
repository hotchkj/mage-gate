// Vision: [MutationSitesFromKills] mirrors [MutationCoverageFromKills]—threshold and scope via kill token.
package gate

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const mutationFilterChangedMarker = "changed"

func testMutationPathFilters(
	excludeSegments []string,
	testFilePatterns []string,
) mutationPathFilters {
	return mutationPathFilters{
		excludeSegments:  append([]string(nil), excludeSegments...),
		testFilePatterns: append([]string(nil), testFilePatterns...),
	}
}

func TestMutationSitesFromKills_PassAndFail(t *testing.T) {
	t.Parallel()
	const json = `{"mutations":[
		{"file":"a.go","package":"p","status":"KILLED"},
		{"file":"a.go","package":"p","status":"KILLED"},
		{"file":"a.go","package":"p","status":"LIVED"}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{stepID: "mk-1", qualityScope: scope, check: result.Check}
	if err := MutationSitesFromKills(out, MaxSites(3)); err != nil {
		t.Fatalf("max 3: %v", err)
	}
	if err := MutationSitesFromKills(out, MaxSites(2)); err == nil {
		t.Fatal("expected failure: 3 sites in a.go, max 2")
	} else if !errors.Is(err, ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestMutationSitesFromKills_ExcludeScopeIgnoresPath(t *testing.T) {
	t.Parallel()
	// 5 mutants under vendor, 1 in pkg — with Exclude("vendor") only pkg/x.go is scored.
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"},
			{"status":"KILLED"},{"status":"KILLED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"KILLED"}]}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scopeOnlyPkg, err := NewQualityScope("./...", Exclude("vendor"))
	if err != nil {
		t.Fatal(err)
	}
	out := MutationKillsOutput{
		stepID:       "mk-scope",
		qualityScope: scopeOnlyPkg,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		check:        result.Check,
	}
	if serr := MutationSitesFromKills(out, MaxSites(1)); serr != nil {
		t.Fatalf("only pkg/x.go counts: %v", serr)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outUnscoped := MutationKillsOutput{stepID: "mk-all", qualityScope: scopeAll, check: result.Check}
	if err := MutationSitesFromKills(outUnscoped, MaxSites(1)); err == nil {
		t.Fatal("expected failure: vendor file still in scope for unscoped check")
	} else if !errors.Is(err, ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestMutationSitesFromKills_TestFilePatternsSkipTestSources(t *testing.T) {
	t.Parallel()
	// Six mutants in a_test.go, one in a.go — max 5: scored without test skip fails on a_test.go.
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"},
			{"status":"KILLED"},{"status":"KILLED"},{"status":"KILLED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"KILLED"}]}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scopeTestSkip, err := NewQualityScope("./...", TestFilePatterns("*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := MutationKillsOutput{
		stepID:       "mk-tf",
		qualityScope: scopeTestSkip,
		pathFilters:  testMutationPathFilters(nil, []string{"*_test.go"}),
		check:        result.Check,
	}
	if serr := MutationSitesFromKills(out, MaxSites(5)); serr != nil {
		t.Fatalf("test file ignored: only a.go (1) counts: %v", serr)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outAll := MutationKillsOutput{stepID: "mk-tf2", qualityScope: scopeAll, check: result.Check}
	if err := MutationSitesFromKills(outAll, MaxSites(5)); err == nil {
		t.Fatal("expected failure: a_test.go counted without TestFilePatterns")
	} else if !errors.Is(err, ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestMutationSitesFromKillsUsesTokenPathFilters(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[{"status":"KILLED"},{"status":"KILLED"}]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"KILLED"}]}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	rawScopeWithoutExclude := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{
		stepID:       "mk-token-filters",
		qualityScope: rawScopeWithoutExclude,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		check:        result.Check,
	}
	if serr := MutationSitesFromKills(out, MaxSites(1)); serr != nil {
		t.Fatalf("persisted token filters should exclude vendor despite raw scope: %v", serr)
	}
}

func TestMutationPathFiltersReturnDefensiveCopies(t *testing.T) {
	t.Parallel()
	filters := testMutationPathFilters([]string{"vendor"}, []string{"*_test.go"})
	excludeSegs, patterns := filters.thresholdPathFilters()
	excludeSegs[0] = mutationFilterChangedMarker
	patterns[0] = mutationFilterChangedMarker
	gotExSegs, gotPatterns := filters.thresholdPathFilters()
	if !reflect.DeepEqual(gotExSegs, []string{"vendor"}) {
		t.Fatalf("exclude segments mutated through returned slice: %v", gotExSegs)
	}
	if !reflect.DeepEqual(gotPatterns, []string{"*_test.go"}) {
		t.Fatalf("test patterns mutated through returned slice: %v", gotPatterns)
	}
}

func TestMutationSitesFromKills_RejectsIncompleteOutput(t *testing.T) {
	t.Parallel()
	t.Run("missingCheck", func(t *testing.T) {
		t.Parallel()
		scope, _ := NewQualityScope("./...")
		err := MutationSitesFromKills(MutationKillsOutput{stepID: "mk-1", qualityScope: scope}, MaxSites(1))
		if err == nil {
			t.Fatal("expected error for missing check")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
	t.Run("emptyStepID", func(t *testing.T) {
		t.Parallel()
		scope, _ := NewQualityScope("./...")
		err := MutationSitesFromKills(
			MutationKillsOutput{qualityScope: scope, check: &gatecheck.MutationKillsCheck{}},
			MaxSites(1),
		)
		if err == nil {
			t.Fatal("expected error for empty stepID")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
	t.Run("missingQualityScope", func(t *testing.T) {
		t.Parallel()
		err := MutationSitesFromKills(
			MutationKillsOutput{stepID: "mk-noscope", check: &gatecheck.MutationKillsCheck{}},
			MaxSites(1),
		)
		if err == nil {
			t.Fatal("expected error for missing quality scope")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
}

func TestMutationSitesFromKills_RequiresMaxSites(t *testing.T) {
	t.Parallel()
	const json = `{"mutations":[{"file":"a.go","package":"p","status":"KILLED"}]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{stepID: "mk-1", qualityScope: scope, check: result.Check}
	if err := MutationSitesFromKills(out, MutationSitesThreshold{}); err == nil {
		t.Fatal("expected validation error for missing MaxSites")
	}
}

func TestMutationSitesFromKillsFailureUsesKillOutputModeForDiagnostics(t *testing.T) {
	t.Parallel()
	const json = `{"mutations":[
		{"file":"a.go","package":"p","status":"KILLED"},
		{"file":"a.go","package":"p","status":"KILLED"},
		{"file":"a.go","package":"p","status":"LIVED"}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scope := mustNewQualityScope(t, "./...")
	base := MutationKillsOutput{stepID: "mk-1", qualityScope: scope, check: result.Check}
	t.Run("silent_token_wraps_diagnostic", func(t *testing.T) {
		t.Parallel()
		silent := base
		silent.outputMode = OutputModeAgent
		err := MutationSitesFromKills(silent, MaxSites(2))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if !errors.As(err, &de) {
			t.Fatalf("silent outputMode: want *DiagnosticError, got %T: %v", err, err)
		}
		if !errors.Is(err, ErrMutationSitesFailed) {
			t.Fatalf("errors.Is must still reach ErrMutationSitesFailed, got %v", err)
		}
	})
	t.Run("zero_mode_stays_verbose_chain", func(t *testing.T) {
		t.Parallel()
		err := MutationSitesFromKills(base, MaxSites(2))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if errors.As(err, &de) {
			t.Fatalf("zero outputMode: expected raw verbose chain, got diagnostic %v", err)
		}
		if !errors.Is(err, ErrMutationSitesFailed) {
			t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
		}
	})
}
