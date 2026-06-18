// Vision: Godog step glue—typed getters for scenario fields so feature steps stay declarative and short.
package steps

import (
	qg "github.com/hotchkj/mage-gate/gate"
)

func (s *scenarioState) minPercent() (float64, bool) {
	v, ok := s.stepOpts["minPercent"].(float64)
	return v, ok
}

func (s *scenarioState) maxScore() (float64, bool) {
	v, ok := s.stepOpts["maxScore"].(float64)
	return v, ok
}

func (s *scenarioState) lintConfigPath() (string, bool) {
	v, ok := s.stepOpts["lintConfig"].(string)
	return v, ok
}

func (s *scenarioState) lintConfigValue() (qg.LintConfigValue, bool) {
	v, ok := s.lintConfigPath()
	if !ok {
		return qg.LintConfigValue{}, false
	}
	return qg.LintConfig(v), true
}

func (s *scenarioState) lintToolValueForStep(step string) (qg.LintToolValue, bool) {
	spec, ok := s.stepToolSpec(step)
	if !ok {
		return qg.LintToolValue{}, false
	}
	return qg.LintToolSpec(spec), true
}

func (s *scenarioState) lintToolchainForStep(step string) (qg.LintToolchain, error) {
	var configPath qg.LintConfigValue
	if v, ok := s.lintConfigValue(); ok {
		configPath = v
	}
	var toolSpec qg.LintToolValue
	if v, ok := s.lintToolValueForStep(step); ok {
		toolSpec = v
	}
	return qg.NewLintToolchain(configPath, toolSpec, s.lintOptions()...)
}

func (s *scenarioState) deadcodeToolValue() (qg.DeadcodeToolValue, bool) {
	spec, ok := s.stepToolSpec(stepDeadcode)
	if !ok {
		return qg.DeadcodeToolValue{}, false
	}
	return qg.DeadcodeToolSpec(spec), true
}

func (s *scenarioState) markdownlintToolValue() (qg.MarkdownLintToolValue, bool) {
	spec, ok := s.stepToolSpec(stepMarkdownlint)
	if !ok {
		return qg.MarkdownLintToolValue{}, false
	}
	return qg.MarkdownLintToolSpec(spec), true
}

func (s *scenarioState) gocycloToolValue() (qg.GocycloToolValue, bool) {
	spec, ok := s.stepToolSpec(stepCrap)
	if !ok {
		return qg.GocycloToolValue{}, false
	}
	return qg.GocycloToolSpec(spec), true
}

func (s *scenarioState) gremlinsToolValueForStep(step string) (qg.GremlinsToolValue, bool) {
	spec, ok := s.stepToolSpec(step)
	if !ok {
		return qg.GremlinsToolValue{}, false
	}
	return qg.GremlinsToolSpec(spec), true
}

func (s *scenarioState) coverageThreshold() qg.CoverageThreshold {
	v, _ := s.minPercent()
	return qg.MinPercent(v)
}

func (s *scenarioState) crapThreshold() qg.CrapThreshold {
	v, _ := s.maxScore()
	return qg.MaxScore(v)
}

func (s *scenarioState) durationThreshold() (qg.DurationThreshold, bool) {
	v, ok := s.stepOpts["maxSeconds"].(float64)
	if !ok {
		return qg.DurationThreshold{}, false
	}
	return qg.MaxSeconds(v), true
}

func (s *scenarioState) mutationSitesThreshold() (qg.MutationSitesThreshold, bool) {
	v, ok := s.stepOpts["maxSites"].(int)
	if !ok {
		return qg.MutationSitesThreshold{}, false
	}
	return qg.MaxSites(v), true
}

func (s *scenarioState) mutationCoverageThreshold() (qg.MutationCoverageThreshold, bool) {
	v, ok := s.stepOpts["mutationCoverageMin"].(int)
	if !ok {
		return qg.MutationCoverageThreshold{}, false
	}
	return qg.MinMutationCoverage(v), true
}

func (s *scenarioState) zzBddWantsHighCoverageMutationFixture() bool {
	v, ok := s.stepOpts["mutationCoverageMin"].(int)
	return ok && v > 0
}

func (s *scenarioState) minKillRate() (int, bool) {
	v, ok := s.stepOpts["minKillRate"].(int)
	return v, ok
}
