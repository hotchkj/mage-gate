package steps

import (
	"strings"

	qg "github.com/hotchkj/mage-gate/gate"
)

func (s *scenarioState) addLintExtraArg(arg string) error {
	s.lintExtraArgs = append(s.lintExtraArgs, arg)
	return nil
}

func (s *scenarioState) addDeadcodeExtraArg(arg string) error {
	s.deadcodeExtraArgs = append(s.deadcodeExtraArgs, arg)
	return nil
}

func (s *scenarioState) addMarkdownlintExtraArg(arg string) error {
	s.markdownlintExtraArgs = append(s.markdownlintExtraArgs, arg)
	return nil
}

func (s *scenarioState) addVetExtraArg(arg string) error {
	s.vetExtraArgs = append(s.vetExtraArgs, arg)
	return nil
}

func (s *scenarioState) addCompileExtraArg(arg string) error {
	s.compileExtraArgs = append(s.compileExtraArgs, arg)
	return nil
}

func (s *scenarioState) addTestExtraArg(arg string) error {
	s.testExtraArgs = append(s.testExtraArgs, arg)
	return nil
}

func (s *scenarioState) addMutationExtraArg(arg string) error {
	s.mutationExtraArgs = append(s.mutationExtraArgs, arg)
	return nil
}

func (s *scenarioState) mutationPassthroughArgs() []string {
	out := make([]string, 0, len(s.mutationExtraArgs))
	for i := 0; i < len(s.mutationExtraArgs); i++ {
		arg := s.mutationExtraArgs[i]
		switch {
		case arg == "-tags" || arg == "--tags":
			if i+1 < len(s.mutationExtraArgs) {
				i++
			}
		case strings.HasPrefix(arg, "-tags="):
		case strings.HasPrefix(arg, "--tags="):
		default:
			out = append(out, arg)
		}
	}
	return out
}

func (s *scenarioState) setPackageScopePattern(pattern string) error {
	s.pkgScopePattern = pattern
	return nil
}

func (s *scenarioState) setQualityScopePattern(pattern string) error {
	s.qualityScopePattern = pattern
	return nil
}

func (s *scenarioState) setQualityScopeExclude(segment string) error {
	if segment != "" {
		s.qualityScopeExcludes = append(s.qualityScopeExcludes, segment)
	}
	return nil
}

func (s *scenarioState) addQualityScopeTag(tag string) error {
	if tag != "" {
		s.qualityScopeTags = append(s.qualityScopeTags, tag)
	}
	return nil
}

func (s *scenarioState) addQualityScopeTestFilePattern(pattern string) error {
	s.qualityScopeTestFilePatterns = append(s.qualityScopeTestFilePatterns, pattern)
	return nil
}

func (s *scenarioState) addCrapExtraArg(arg string) error {
	s.crapExtraArgs = append(s.crapExtraArgs, arg)
	return nil
}

func (s *scenarioState) lintOptions() []qg.LintOption {
	var opts []qg.LintOption
	if customGCLPath, hasPath := s.stepOpts["customGCLPath"].(string); hasPath && customGCLPath != "" {
		opts = append(opts, qg.CustomGCL(customGCLPath))
	}
	if customSpec, hasSpec := s.stepOpts["customLintSpec"].(string); hasSpec && customSpec != "" {
		opts = append(opts, qg.CustomLintToolSpec(customSpec))
	}
	if len(s.lintExtraArgs) > 0 {
		opts = append(opts, qg.LintArgs(s.lintExtraArgs...))
	}
	return opts
}

func (s *scenarioState) deadcodeOptions() []qg.DeadcodeOption {
	if len(s.deadcodeExtraArgs) == 0 {
		return nil
	}
	return []qg.DeadcodeOption{qg.DeadcodeArgs(s.deadcodeExtraArgs...)}
}

func (s *scenarioState) markdownlintOptions() []qg.MarkdownLintOption {
	if len(s.markdownlintExtraArgs) == 0 {
		return nil
	}
	return []qg.MarkdownLintOption{qg.MarkdownLintArgs(s.markdownlintExtraArgs...)}
}

func (s *scenarioState) vetOptions() []qg.VetOption {
	if len(s.vetExtraArgs) == 0 {
		return nil
	}
	return []qg.VetOption{qg.VetArgs(s.vetExtraArgs...)}
}

func (s *scenarioState) compileOptions() []qg.CompileOption {
	if len(s.compileExtraArgs) == 0 {
		return nil
	}
	return []qg.CompileOption{qg.CompileArgs(s.compileExtraArgs...)}
}

func (s *scenarioState) testOptions() []qg.TestOption {
	if len(s.testExtraArgs) == 0 {
		return nil
	}
	return []qg.TestOption{qg.TestArgs(s.testExtraArgs...)}
}

func (s *scenarioState) mutationOptions() []qg.MutationOption {
	if len(s.mutationExtraArgs) == 0 {
		return nil
	}
	return []qg.MutationOption{qg.MutationArgs(s.mutationExtraArgs...)}
}

func (s *scenarioState) crapOptions() []qg.CrapOption {
	if len(s.crapExtraArgs) == 0 {
		return nil
	}
	return []qg.CrapOption{qg.CrapArgs(s.crapExtraArgs...)}
}
