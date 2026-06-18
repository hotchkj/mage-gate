// Vision: QualityScope defines what the gate evaluates — not what you ship.
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// QualityScope is the measurement boundary for coverage/CRAP/mutation: package pattern seeds coverpkg; [Tags]
// select tagged production files; [Exclude] and [TestFilePatterns] filter which import paths/files count. It does
// not select packages for [Lint]/[Vet]/[Compile]/[Deadcode]. Zero value is invalid; use [NewQualityScope].
//
// QualityScope is an opaque value token (~8 bytes): it holds a pointer to immutable private data.
// Pass by value throughout the public API; pointer passing would imply mutability the type deliberately forbids.
type QualityScope struct {
	data *qualityScopeData
}

// qualityScopeData holds the actual scope state. Only NewQualityScope creates this.
type qualityScopeData struct {
	packages         string
	tags             []string
	excludeSegments  []string
	testFilePatterns []string
}

// qualityScopeConfig holds optional configuration for [QualityScope]. Options mutate this
// private struct so they cannot access or overwrite the required package identity.
type qualityScopeConfig struct {
	tags             []string
	excludeSegments  []string
	testFilePatterns []string
}

// QualityScopeOption configures optional modifiers on a [QualityScope]. Typed option functions
// exist for scope modifiers that carry invariants (tags, excludes, test-file patterns).
// Generic tool argv uses *Args constructors instead.
type QualityScopeOption func(*qualityScopeConfig)

// NewQualityScope constructs a QualityScope with the given package pattern.
// pkgs must not be empty. Returns an error if pkgs is empty, whitespace-only, or starts with "-".
func NewQualityScope(pkgs string, opts ...QualityScopeOption) (QualityScope, error) {
	trimmed, err := validateGoTestPackagePattern(pkgs, ErrQualityScopeEmpty)
	if err != nil {
		return QualityScope{}, err
	}
	var cfg qualityScopeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return QualityScope{
		data: &qualityScopeData{
			packages:         trimmed,
			tags:             cfg.tags,
			excludeSegments:  cfg.excludeSegments,
			testFilePatterns: cfg.testFilePatterns,
		},
	}, nil
}

// Packages returns the Go package pattern that scopes quality checks.
// Returns empty string for zero-value QualityScope (invalid state).
func (s QualityScope) Packages() string {
	if s.data == nil {
		return ""
	}
	return s.data.packages
}

// Tags returns a copy of the build tags that are part of this quality scope.
// Returns nil for zero-value QualityScope.
func (s QualityScope) Tags() []string {
	if s.data == nil {
		return nil
	}
	return append([]string(nil), s.data.tags...)
}

// Tags appends build tags to the production quality scope.
func Tags(tags ...string) QualityScopeOption {
	return func(cfg *qualityScopeConfig) {
		for _, tag := range tags {
			if tag != "" {
				cfg.tags = append(cfg.tags, tag)
			}
		}
	}
}

// Exclude appends path segments to exclude from the production quality scope.
// Packages whose import paths contain any segment are excluded from coverage,
// CRAP, and mutation analysis.
func Exclude(segments ...string) QualityScopeOption {
	return func(cfg *qualityScopeConfig) {
		for _, seg := range segments {
			if seg != "" {
				cfg.excludeSegments = append(cfg.excludeSegments, seg)
			}
		}
	}
}

// TestFilePatterns sets filename glob patterns identifying test files within
// production packages (e.g. "*_test.go"). Steps that analyze individual files
// (such as Crap's gocyclo pass) skip matching files from production metrics.
// Callers supply the patterns; the library applies no default.
func TestFilePatterns(patterns ...string) QualityScopeOption {
	return func(cfg *qualityScopeConfig) {
		for _, pat := range patterns {
			if pat != "" {
				cfg.testFilePatterns = append(cfg.testFilePatterns, pat)
			}
		}
	}
}

// ExcludeSegments returns a copy of the exclude path segments.
// Returns nil for zero-value QualityScope.
func (s QualityScope) ExcludeSegments() []string {
	if s.data == nil {
		return nil
	}
	return append([]string(nil), s.data.excludeSegments...)
}

// TestFilePatterns returns a copy of the test file patterns.
// Returns nil for zero-value QualityScope.
func (s QualityScope) TestFilePatterns() []string {
	if s.data == nil {
		return nil
	}
	return append([]string(nil), s.data.testFilePatterns...)
}

// qualityScopePackages returns the raw packages string for internal use.
// Returns empty string for zero-value QualityScope.
func qualityScopePackages(qs QualityScope) string {
	if qs.data == nil {
		return ""
	}
	return qs.data.packages
}

// qualityScopeTags returns the raw tags slice for internal use.
// Returns nil for zero-value QualityScope.
func qualityScopeTags(qs QualityScope) []string {
	if qs.data == nil {
		return nil
	}
	return qs.data.tags
}

// qualityScopeExcludeSegments returns the raw exclude segments slice for internal use.
// Returns nil for zero-value QualityScope.
func qualityScopeExcludeSegments(qs QualityScope) []string {
	if qs.data == nil {
		return nil
	}
	return qs.data.excludeSegments
}

// qualityScopeFingerprint returns a deterministic SHA-256 hex digest of the scope's full identity:
// packages, sorted tags, sorted exclude segments, sorted test-file patterns.
// Returns empty string for zero-value QualityScope.
func qualityScopeFingerprint(qs QualityScope) string {
	if qs.data == nil {
		return ""
	}
	tags := append([]string(nil), qs.data.tags...)
	sort.Strings(tags)
	excludes := append([]string(nil), qs.data.excludeSegments...)
	sort.Strings(excludes)
	patterns := append([]string(nil), qs.data.testFilePatterns...)
	sort.Strings(patterns)
	canonical := fmt.Sprintf("%s\x00%s\x00%s\x00%s",
		qs.data.packages,
		strings.Join(tags, "\x00"),
		strings.Join(excludes, "\x00"),
		strings.Join(patterns, "\x00"),
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func joinSorted(s []string) string {
	return strings.Join(s, ",")
}
