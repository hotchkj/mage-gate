// Vision: Mutation scan setup and consumer dispatch helpers (keeps step_execution.go under file limits).
package steps

import (
	"context"
	"fmt"

	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

func (s *scenarioState) buildQualityScope() (qg.QualityScope, error) {
	var qualityOpts []qg.QualityScopeOption
	for _, seg := range s.qualityScopeExcludes {
		if seg != "" {
			qualityOpts = append(qualityOpts, qg.Exclude(seg))
		}
	}
	for _, tag := range s.qualityScopeTags {
		if tag != "" {
			qualityOpts = append(qualityOpts, qg.Tags(tag))
		}
	}
	for _, p := range s.qualityScopeTestFilePatterns {
		qualityOpts = append(qualityOpts, qg.TestFilePatterns(p))
	}
	return qg.NewQualityScope(s.qualityScopePattern, qualityOpts...)
}

func (s *scenarioState) runMutationScanStep(
	ctx context.Context,
	display qg.CommandRunner,
	resolver qg.ToolResolver,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
) error {
	gremlinsVal, gOk := s.gremlinsToolValueForStep(stepMutationScan)
	if !gOk {
		gremlinsVal = qg.GremlinsToolValue{}
	}
	inv, _ := s.priorQualityScopeInventoryOutput()
	mr, err := qg.NewMutationRunner(display, resolver, store, mem)
	if err != nil {
		return err
	}
	scanOut, scanErr := mr.Scan(ctx, root, scope, inv, gremlinsVal, s.mutationOptions()...)
	s.lastMutationScanOut = scanOut
	s.mutationScanOutputReady = scanErr == nil
	return scanErr
}

func (s *scenarioState) dispatchMutationSitesStep() error {
	if !s.mutationScanOutputReady {
		return errMutationScanArtifactsMissing
	}
	threshold, ok := s.mutationSitesThreshold()
	if !ok {
		threshold = qg.MutationSitesThreshold{}
	}
	return qg.MutationSites(s.lastMutationScanOut, threshold)
}

func (s *scenarioState) dispatchMutationCoverageStep() error {
	if !s.mutationScanOutputReady {
		return errMutationScanArtifactsMissing
	}
	covTh, ok := s.mutationCoverageThreshold()
	if !ok {
		covTh = qg.MutationCoverageThreshold{}
	}
	return qg.MutationCoverage(s.lastMutationScanOut, covTh)
}

// givenFixtureTestFile registers a test go file in the in-memory module map for go list test basenames
// and writes the file to the fake workspace (root-relative path).
//
//nolint:mnd // 0700/0600 are in-memory fake FS perms, not user-facing magic numbers
func (s *scenarioState) givenFixtureTestFile(rel string) error {
	rel = fsnorm.Canonical(rel)
	mem := s.ensureMem()
	if err := mem.MkdirAll(fsnorm.Dir(rel), 0o700); err != nil {
		return err
	}
	if err := mem.WriteFile(rel, []byte("package t\n"), 0o600); err != nil {
		return err
	}
	base := fsnorm.Base(rel)
	parent := fsnorm.Dir(rel)
	if len(s.modulePackages) == 0 {
		return errFixtureTestRegisterPackages
	}
	for imp, info := range s.modulePackages {
		rr, err := s.rootRelPackageDir(info.Dir)
		if err != nil {
			return err
		}
		if rr == parent {
			info.Test = append(append([]string(nil), info.Test...), base)
			s.modulePackages[imp] = info
			return nil
		}
	}
	return fmt.Errorf("%w: %q", errFixtureTestPackageUnmapped, rel)
}

// givenFixtureSourceFile registers a non-test Go file in the in-memory module map for
// mutation inventory and writes the file to the fake workspace (root-relative path).
//
//nolint:mnd // 0700/0600 are in-memory fake FS perms, not user-facing magic numbers
func (s *scenarioState) givenFixtureSourceFile(rel string) error {
	rel = fsnorm.Canonical(rel)
	mem := s.ensureMem()
	if err := mem.MkdirAll(fsnorm.Dir(rel), 0o700); err != nil {
		return err
	}
	if err := mem.WriteFile(rel, []byte("package p\n"), 0o600); err != nil {
		return err
	}
	base := fsnorm.Base(rel)
	parent := fsnorm.Dir(rel)
	if len(s.modulePackages) == 0 {
		return errFixtureTestRegisterPackages
	}
	for imp, info := range s.modulePackages {
		rr, err := s.rootRelPackageDir(info.Dir)
		if err != nil {
			return err
		}
		if rr == parent {
			info.GoFiles = appendUniqueString(info.GoFiles, base)
			s.modulePackages[imp] = info
			return nil
		}
	}
	return fmt.Errorf("%w: %q", errFixtureTestPackageUnmapped, rel)
}

//nolint:mnd // 0700/0600 are in-memory fake FS perms, not user-facing magic numbers
func (s *scenarioState) givenFixtureUnlistedSourceFile(rel string) error {
	rel = fsnorm.Canonical(rel)
	mem := s.ensureMem()
	if err := mem.MkdirAll(fsnorm.Dir(rel), 0o700); err != nil {
		return err
	}
	return mem.WriteFile(rel, []byte("package unlisted\n"), 0o600)
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
