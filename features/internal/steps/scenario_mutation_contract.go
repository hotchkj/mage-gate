// Vision: Build non-literal gremlins argv requirements for BDD fakes: coverpkg, tags, and arg order.
package steps

import (
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func commonModuleRootFromDirs(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	if len(dirs) == 1 {
		return fsnorm.Dir(dirs[0])
	}
	common := fsnorm.Canonical(dirs[0])
	for _, o := range dirs[1:] {
		o = fsnorm.Canonical(o)
		for common != "" && o != common && !strings.HasPrefix(o, common+"/") {
			if i := strings.LastIndex(common, "/"); i >= 0 {
				common = common[:i]
			} else {
				return ""
			}
		}
	}
	return common
}

func (s *scenarioState) moduleSourceRoot() string {
	if root := s.moduleRootFromPackageDirs(); root != "" {
		return root
	}
	return commonModuleRootFromDirs(s.allPackageAbsDirs())
}

func (s *scenarioState) moduleRootFromPackageDirs() string {
	if s.modulePath == "" {
		return ""
	}
	for importPath, info := range s.modulePackages {
		if root := moduleRootFromPackageDir(s.modulePath, importPath, info.Dir); root != "" {
			return root
		}
	}
	return moduleRootFromPackageDir(s.modulePath, s.pkgImport, s.pkgDir)
}

func moduleRootFromPackageDir(modulePath, importPath, dir string) string {
	if modulePath == "" || importPath == "" || dir == "" {
		return ""
	}
	if importPath != modulePath && !strings.HasPrefix(importPath, modulePath+"/") {
		return ""
	}
	relImport := strings.TrimPrefix(importPath, modulePath)
	relImport = strings.TrimPrefix(fsnorm.Canonical(relImport), "/")
	dir = strings.TrimRight(fsnorm.Canonical(dir), "/")
	if relImport == "" {
		return dir
	}
	suffix := "/" + relImport
	if !strings.HasSuffix(dir, suffix) {
		return ""
	}
	return strings.TrimRight(dir[:len(dir)-len(suffix)], "/")
}

func (s *scenarioState) allPackageAbsDirs() []string {
	if len(s.modulePackages) == 0 {
		if s.pkgDir == "" {
			return nil
		}
		return []string{s.pkgDir}
	}
	d := make([]string, 0, len(s.modulePackages))
	for _, info := range s.modulePackages {
		d = append(d, info.Dir)
	}
	return d
}

func (s *scenarioState) rootRelPackageDir(absDir string) (string, error) {
	base := s.moduleSourceRoot()
	if base == "" {
		return "", errModuleSourceNotDerivable
	}
	rel, err := fsnorm.Rel(base, absDir)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func (s *scenarioState) qualityScopeImportKeys() []string {
	keys := make([]string, 0, len(s.modulePackages))
	for k := range s.modulePackages {
		if s.packageMatchesQualityScope(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *scenarioState) packageMatchesQualityScope(importPath string) bool {
	if strings.TrimSpace(s.qualityScopePattern) == "" {
		return true
	}
	pattern := fsnorm.Canonical(s.qualityScopePattern)
	if pattern == "..." {
		return true
	}
	if !strings.HasSuffix(pattern, "/...") {
		prefix := strings.TrimPrefix(pattern, "./")
		prefix = fsnorm.Canonical(prefix)
		info, ok := s.modulePackages[importPath]
		if !ok {
			return false
		}
		rel, err := s.rootRelPackageDir(info.Dir)
		if err != nil {
			return false
		}
		return rel == prefix
	}
	prefix := strings.TrimPrefix(pattern, "./")
	prefix = strings.TrimSuffix(prefix, "/...")
	prefix = fsnorm.Canonical(prefix)
	if prefix == "" {
		return false
	}
	info, ok := s.modulePackages[importPath]
	if !ok {
		return false
	}
	rel, err := s.rootRelPackageDir(info.Dir)
	if err != nil {
		return false
	}
	return rel == prefix || strings.HasPrefix(rel, prefix+"/")
}

// expectedCoverpkgArg returns a single argv fragment "--coverpkg=import,..." for the fake gremlins check.
func (s *scenarioState) expectedCoverpkgArg() (string, error) {
	keys := s.qualityScopeImportKeys()
	var excludes []string
	for _, segment := range s.qualityScopeExcludes {
		excludes = append(excludes, gatecheck.ParseExcludeSegments(segment)...)
	}
	keys = gatecheck.FilterCoverpkg(keys, excludes)
	if len(keys) == 0 {
		return "", errNoQualityScopePackageMatches
	}
	return "--coverpkg=" + strings.Join(keys, ","), nil
}

func (s *scenarioState) mutationScanRequiredArgSequence() ([]string, error) {
	coverArg, err := s.expectedCoverpkgArg()
	if err != nil {
		return nil, err
	}
	seq := []string{coverArg}
	if tagArg := s.expectedMutationTagsArg(); tagArg != "" {
		seq = append(seq, tagArg)
	}
	seq = append(seq, "--dry-run")
	seq = append(seq, s.mutationPassthroughArgs()...)
	return seq, nil
}

func (s *scenarioState) mutationKillsRequiredArgSequence() ([]string, error) {
	coverArg, err := s.expectedCoverpkgArg()
	if err != nil {
		return nil, err
	}
	seq := []string{coverArg}
	if tagArg := s.expectedMutationTagsArg(); tagArg != "" {
		seq = append(seq, tagArg)
	}
	seq = append(seq, s.mutationPassthroughArgs()...)
	return seq, nil
}

func (s *scenarioState) expectedMutationTagsArg() string {
	var merged []string
	seen := make(map[string]struct{})
	for _, tag := range s.qualityScopeTags {
		merged = appendScenarioBuildTags(merged, seen, tag)
	}
	merged = s.appendMutationArgTags(merged, seen)
	if len(merged) == 0 {
		return ""
	}
	return "--tags=" + strings.Join(merged, ",")
}

func (s *scenarioState) appendMutationArgTags(merged []string, seen map[string]struct{}) []string {
	for argIndex := 0; argIndex < len(s.mutationExtraArgs); argIndex++ {
		arg := s.mutationExtraArgs[argIndex]
		switch {
		case arg == "-tags" || arg == "--tags":
			if argIndex+1 < len(s.mutationExtraArgs) {
				merged = appendScenarioBuildTags(merged, seen, s.mutationExtraArgs[argIndex+1])
				argIndex++
			}
		case strings.HasPrefix(arg, "-tags="):
			merged = appendScenarioBuildTags(merged, seen, strings.TrimPrefix(arg, "-tags="))
		case strings.HasPrefix(arg, "--tags="):
			merged = appendScenarioBuildTags(merged, seen, strings.TrimPrefix(arg, "--tags="))
		}
	}
	return merged
}

func appendScenarioBuildTags(merged []string, seen map[string]struct{}, value string) []string {
	for _, tag := range splitScenarioBuildTags(value) {
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}
	return merged
}

func splitScenarioBuildTags(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
}
