// Vision: BDD adapters for MutationCoverageFromKills and MutationSitesFromKills.
package steps

import (
	"errors"
	"fmt"

	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

var (
	errMutationFixturePathMissing     = errors.New("mutation fixture path missing from raw artifact")
	errMutationFixturePathNotExcluded = errors.New("mutation fixture path remained in scoped evaluation")
)

func (s *scenarioState) runEvaluateMutationCoverageFromFullRunArtifacts() error {
	callsBefore := len(s.commandLog())
	out, ok := s.stepOpts["mutationKillsOutput"].(qg.MutationKillsOutput)
	if !ok {
		return fmt.Errorf("%w", errNoMutationKillsOutput)
	}
	th, ok := s.mutationCoverageThreshold()
	if !ok {
		th = qg.MutationCoverageThreshold{}
	}
	s.runErr = qg.MutationCoverageFromKills(out, th)
	return s.assertNoAdapterDispatch(callsBefore, "mutationcoverage-from-kills")
}

func (s *scenarioState) runEvaluateMutationSitesFromFullRunArtifacts() error {
	callsBefore := len(s.commandLog())
	out, ok := s.stepOpts["mutationKillsOutput"].(qg.MutationKillsOutput)
	if !ok {
		return fmt.Errorf("%w", errNoMutationKillsOutput)
	}
	th, ok := s.mutationSitesThreshold()
	if !ok {
		th = qg.MutationSitesThreshold{}
	}
	s.runErr = qg.MutationSitesFromKills(out, th)
	return s.assertNoAdapterDispatch(callsBefore, "mutationsites-from-kills")
}

func (s *scenarioState) assertNoAdapterDispatch(callsBefore int, adapter string) error {
	callsAfter := s.commandLog()
	if len(callsAfter) < callsBefore {
		return fmt.Errorf("%w: adapter %q command log shrank", errUnexpectedDispatch, adapter)
	}
	if len(callsAfter) != callsBefore {
		return fmt.Errorf("%w: adapter %q got %v", errUnexpectedDispatch, adapter, callsAfter[callsBefore:])
	}
	return nil
}

func (s *scenarioState) assertMutationCoverageEvaluationDidNotInclude(relPath string) error {
	if _, ok := s.stepOpts["mutationKillsOutput"].(qg.MutationKillsOutput); ok {
		return s.assertFilteredKillsArtifactExcludesPath(relPath, "mutation coverage evaluation")
	}
	stepID, found := s.ensureStore().FindArtifactByStepPrefix(stepMutationScan, "mutations.json")
	if found {
		return s.assertFilteredStoreMutationExcludesPath(stepID, relPath, "mutation coverage evaluation")
	}
	return s.assertFilteredKillsArtifactExcludesPath(relPath, "mutation coverage evaluation")
}

func (s *scenarioState) assertMutationKillsEvaluationDidNotInclude(relPath string) error {
	return s.assertFilteredKillsArtifactExcludesPath(relPath, "mutation-kills scoped evaluation")
}

func (s *scenarioState) assertFilteredKillsArtifactExcludesPath(relPath, label string) error {
	store := s.ensureStore()
	stepID, found := store.FindArtifactByStepPrefix(stepMutationKills, "mutations.json")
	if !found {
		return fmt.Errorf("%w: mutations.json from mutationkills step", errArtifactMissing)
	}
	return s.assertFilteredStoreMutationExcludesPath(stepID, relPath, label)
}

func (s *scenarioState) assertFilteredStoreMutationExcludesPath(stepID, relPath, label string) error {
	want := fsnorm.Canonical(relPath)
	store := s.ensureStore()
	data, rerr := store.Read(stepID, "mutations.json")
	if rerr != nil {
		return rerr
	}
	result, perr := gatecheck.MutationKills(data, 0)
	if perr != nil {
		return perr
	}
	foundRaw := false
	for _, f := range result.Check.Files {
		if fsnorm.Canonical(f.File) != want {
			continue
		}
		foundRaw = true
		break
	}
	if !foundRaw {
		return fmt.Errorf("%w: label=%s path=%q", errMutationFixturePathMissing, label, relPath)
	}
	scoped := gatecheck.FilterMutationMetricsByQualityScope(
		gatecheck.SnapshotFromMutationKillsCheck(result.Check),
		s.qualityScopeExcludes,
		s.qualityScopeTestFilePatterns,
	)
	for _, fileMetrics := range scoped.Files {
		if fsnorm.Canonical(fileMetrics.File) == want {
			return fmt.Errorf("%w: label=%s path=%q", errMutationFixturePathNotExcluded, label, relPath)
		}
	}
	if len(scoped.Files) == len(result.Check.Files) {
		return fmt.Errorf("%w: label=%s path=%q", errMutationFixturePathNotExcluded, label, relPath)
	}
	return nil
}
