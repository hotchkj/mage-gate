// Vision: CRAP index from paired gocyclo-style complexity lines and coverage percentages (pure numeric merge).
package gatecheck

import (
	"bufio"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

var (
	errCrapMaxNotPositive = errors.New("maxCrap must be positive")
	errGocycloFields      = errors.New("gocyclo line has fewer than 4 fields")
	errGocycloComplexity  = errors.New("gocyclo complexity field is not an integer")
	errCoverFuncPercent   = errors.New("cover -func coverage percent invalid")
	coverFuncLineRe       = regexp.MustCompile(`^(.+):(\d+):\s+(\S+)\s+([\d.]+)%$`)
)

// CrapOffender represents a function that exceeds the CRAP threshold.
type CrapOffender struct {
	Path       string
	Name       string
	Complexity int
	Coverage   float64
	Crap       float64
}

// CrapResult holds the result of a CRAP check.
type CrapResult struct {
	Passed    bool
	Offenders []CrapOffender
	MaxCrap   float64
}

// Crap calculates CRAP scores from gocyclo and cover-func output.
// CRAP = complexity^2 * (1 - coverage/100)^3 + complexity
// testFilePatterns are filename glob patterns (e.g. "*_test.go") whose
// matches are excluded from the gocyclo complexity map; the coverage
// profile already omits them.
// Returns error if inputs are invalid or cannot be parsed.
func Crap(
	gocycloOutput, coverFuncOutput, module, moduleDir string,
	maxCrap float64,
	testFilePatterns []string,
) (CrapResult, error) {
	if maxCrap <= 0 {
		return CrapResult{}, errCrapMaxNotPositive
	}

	complexity, err := parseGocycloOutput(gocycloOutput, moduleDir, testFilePatterns)
	if err != nil {
		return CrapResult{}, fmt.Errorf("%w: %w", ErrCrapFailed, err)
	}
	coverage, err := parseCoverFuncOutput(coverFuncOutput, module)
	if err != nil {
		return CrapResult{}, fmt.Errorf("%w: %w", ErrCrapFailed, err)
	}

	var offenders []CrapOffender
	for funcKey, comp := range complexity {
		cov := coverage[funcKey]
		crap := calculateCrap(comp, cov)
		if crap >= maxCrap {
			offenders = append(offenders, CrapOffender{
				Path:       funcKey[0],
				Name:       funcKey[1],
				Complexity: comp,
				Coverage:   cov,
				Crap:       crap,
			})
		}
	}
	sortCrapOffenders(offenders)

	return CrapResult{
		Passed:    len(offenders) == 0,
		Offenders: offenders,
		MaxCrap:   maxCrap,
	}, nil
}

// FormatCrapReport formats the CRAP result as a human-readable report.
func FormatCrapReport(result CrapResult) string {
	if result.Passed {
		return fmt.Sprintf("CRAP check passed (max %.1f)", result.MaxCrap)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CRAP check failed: %d function(s) exceed threshold of %.1f\n", len(result.Offenders), result.MaxCrap)
	for _, o := range result.Offenders {
		fmt.Fprintf(&sb, "  %s:%s - CRAP=%.1f (complexity=%d, coverage=%.1f%%)\n",
			o.Path, o.Name, o.Crap, o.Complexity, o.Coverage)
	}
	return sb.String()
}

func calculateCrap(complexity int, coverage float64) float64 {
	c := float64(complexity)
	return c*c*(1-coverage/100)*(1-coverage/100)*(1-coverage/100) + c
}

// normalizeFilePath normalizes a tool path toward root-relative canonical form for merging gocyclo with cover output.
// It uses [fsnorm.Canonical]; absolute paths strip a matching module-root prefix where present.
func normalizeFilePath(filePath, moduleRoot string) string {
	normalizedPath := fsnorm.Canonical(filePath)
	normalizedRoot := fsnorm.Canonical(moduleRoot)

	if isLexicallyAbsolute(filePath, normalizedPath) {
		if normalizedRoot != "" && strings.HasPrefix(normalizedPath, normalizedRoot) {
			suffix := normalizedPath[len(normalizedRoot):]
			if suffix == "" || suffix[0] == '/' {
				return strings.TrimPrefix(suffix, "/")
			}
		}
		return normalizedPath
	}

	return normalizedPath
}

func parseGocycloOutput(output, moduleDir string, testFilePatterns []string) (map[[2]string]int, error) {
	result := make(map[[2]string]int)
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// gocyclo output format: "complexity package funcName file:line:col"
		// Example: "5 harness Validate internal/harness/config.go:10:1"
		parts := strings.Fields(line)
		if len(parts) < 4 { //nolint:mnd // minimum fields: complexity, package, funcName, file:line:col
			return nil, fmt.Errorf("%w: %q", errGocycloFields, line)
		}
		complexity, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: %w (line %q)", errGocycloComplexity, err, line)
		}
		funcName := parts[2]
		// Last part is file:line:col
		filePart := parts[len(parts)-1]
		filePath := fsnorm.Canonical(extractFilePath(filePart))

		if matchesTestFilePattern(filePath, testFilePatterns) {
			continue
		}

		// Normalize path for cross-platform matching
		normalizedPath := normalizeFilePath(filePath, moduleDir)

		// gocyclo emits receiver-qualified names like "(*Config).Validate"
		// while cover-func emits just "Validate". Strip the receiver so the
		// keys align when merging complexity with coverage.
		funcName = stripReceiver(funcName)

		result[[2]string{normalizedPath, funcName}] = complexity
	}
	return result, nil
}

// extractFilePath strips :line:col suffix from a "file:line:col" token.
func extractFilePath(fileLineCol string) string {
	filePath := strings.TrimSpace(fileLineCol)
	if idx := strings.LastIndex(filePath, ":"); idx > 0 {
		filePath = filePath[:idx]
		if idx2 := strings.LastIndex(filePath, ":"); idx2 > 0 && isAllDigits(filePath[idx2+1:]) {
			return filePath[:idx2]
		}
	}
	return filePath
}

// stripReceiver normalizes "(*Type).Method" or "(Type).Method" to "Method".
func stripReceiver(name string) string {
	if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
		return name[dotIdx+1:]
	}
	return name
}

// matchesTestFilePattern returns true if the file's basename matches any
// of the caller-supplied glob patterns (e.g. "*_test.go").
// Basename uses canonical logical rules ([fsnorm.Base]) so mixed or backslash paths
// from gocyclo agree with patterns on every GOOS.
func matchesTestFilePattern(filePath string, patterns []string) bool {
	base := fsnorm.Base(filePath)
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

func stripModulePrefixFromCoverPath(normalizedPath, module string) string {
	if module == "" {
		return normalizedPath
	}
	modulePrefix := fsnorm.Canonical(module)
	if !strings.HasSuffix(modulePrefix, "/") {
		modulePrefix += "/"
	}
	return strings.TrimPrefix(normalizedPath, modulePrefix)
}

// parseCoverFuncLine parses one cover -func line. skip is true when the line should be ignored
// (no regex match). A parse error is returned only for an invalid coverage percentage token.
func parseCoverFuncLine(
	line string, re *regexp.Regexp, module string,
) (key [2]string, pct float64, skip bool, err error) {
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return [2]string{}, 0, true, nil
	}
	coverage, perr := strconv.ParseFloat(matches[4], 64)
	if perr != nil {
		return [2]string{}, 0, false, fmt.Errorf("%w: %w (line %q)", errCoverFuncPercent, perr, line)
	}
	filePath := matches[1]
	funcName := matches[3]
	normalizedPath := stripModulePrefixFromCoverPath(fsnorm.Canonical(filePath), module)
	return [2]string{normalizedPath, funcName}, coverage, false, nil
}

func parseCoverFuncOutput(output, module string) (map[[2]string]float64, error) {
	result := make(map[[2]string]float64)
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "total:") {
			continue
		}
		key, pct, skip, err := parseCoverFuncLine(line, coverFuncLineRe, module)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		result[key] = pct
	}
	return result, nil
}
