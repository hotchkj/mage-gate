package steps

import (
	"github.com/hotchkj/mage-gate/cmdrunner"
	qg "github.com/hotchkj/mage-gate/gate"
)

func (s *scenarioState) resetOutputAndCommandState() {
	s.responses = nil
	s.outputMode = qg.OutputModeAgent
	s.output.Reset()
	s.runErr = nil
	s.stepName = ""
	s.stepCallsMap = make(map[string][][]cmdrunner.Command)
	s.recordedCalls = nil
	s.allDispatchedCalls = nil
}

func (s *scenarioState) resetContextState() {
	s.responses = nil
	s.stepOpts = make(map[string]interface{})
	s.mem = nil
	s.store = nil
	s.gocycloScores = make(map[string]int)
	s.modulePath = ""
	s.pkgImport = ""
	s.pkgDir = ""
	s.modulePackages = nil
	s.stepsRan = nil
	s.lastMutationScanOut = qg.MutationScanOutput{}
	s.mutationScanOutputReady = false
	s.gremlinsDryRunInvocations = 0
	s.lastArtifact = ""
	s.lastArtifactID = ""
}

func (s *scenarioState) resetFailureState() {
	s.lintClean = false
	s.lintDirty = false
	s.compileClean = false
	s.vetIssues = false
	s.compileFails = false
	s.testFails = false
	s.deadcodeIssues = false
	s.markdownlintIssues = false
	s.mutationExcessive = false
	s.slowTests = false
	s.fastTestsSlowWall = false
	s.packageTestEvents = nil
}

func (s *scenarioState) resetStepState() {
	s.localToolStates = nil
	s.lintExtraArgs = nil
	s.deadcodeExtraArgs = nil
	s.markdownlintExtraArgs = nil
	s.vetExtraArgs = nil
	s.compileExtraArgs = nil
	s.testExtraArgs = nil
	s.mutationExtraArgs = nil
	s.crapExtraArgs = nil
}

func (s *scenarioState) resetMutationState() {
	s.mutationKillsMinRate = 0
	s.mutationKillsResult = nil
	s.pkgScopePattern = ""
	s.qualityScopePattern = ""
	s.qualityScopeTags = nil
	s.qualityScopeExcludes = nil
	s.qualityScopeTestFilePatterns = nil
}
