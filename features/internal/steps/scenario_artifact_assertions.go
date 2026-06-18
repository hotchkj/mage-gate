// Vision: Artifact/provenance Then steps split from scenario_state to keep scenario_state under file-length limits.
package steps

import "fmt"

func (s *scenarioState) assertArtifactFromStep(
	name, stepName string,
) error {
	store := s.ensureStore()
	stepID, found := store.FindArtifactByStepPrefix(stepName, name)
	if !found {
		return fmt.Errorf(
			"%w: %q from step %q",
			errArtifactMissing, name, stepName,
		)
	}
	s.lastArtifact = name
	s.lastArtifactID = stepID
	return nil
}

func (s *scenarioState) assertProvenanceTool(tool string) error {
	if s.lastArtifactID == "" || s.lastArtifact == "" {
		return fmt.Errorf(
			"%w: no prior artifact assertion", errProvenanceMissing,
		)
	}
	store := s.ensureStore()
	prov, err := store.ReadProvenance(s.lastArtifactID, s.lastArtifact)
	if err != nil {
		return fmt.Errorf("%w: %w", errProvenanceMissing, err)
	}
	if prov.Tool != tool {
		return fmt.Errorf(
			"%w: tool = %q, want %q", errProvenanceMissing, prov.Tool, tool,
		)
	}
	return nil
}

func (s *scenarioState) assertProvenanceScope() error {
	if s.lastArtifactID == "" || s.lastArtifact == "" {
		return fmt.Errorf(
			"%w: no prior artifact assertion", errProvenanceMissing,
		)
	}
	store := s.ensureStore()
	prov, err := store.ReadProvenance(s.lastArtifactID, s.lastArtifact)
	if err != nil {
		return fmt.Errorf("%w: %w", errProvenanceMissing, err)
	}
	if prov.Packages != s.pkgScopePattern {
		return fmt.Errorf(
			"%w: packages = %q, want %q",
			errProvenanceMissing, prov.Packages, s.pkgScopePattern,
		)
	}
	return nil
}

func (s *scenarioState) assertStepDidNotExecute(
	stepName string,
) error {
	for _, ran := range s.stepsRan {
		if ran == stepName {
			return fmt.Errorf(
				"%w: %q", errStepDidExecute, stepName,
			)
		}
	}
	return nil
}

func (s *scenarioState) assertFollowingStepsDoNotRun(first, second string) error {
	if err := s.assertStepDidNotExecute(first); err != nil {
		return err
	}
	return s.assertStepDidNotExecute(second)
}
