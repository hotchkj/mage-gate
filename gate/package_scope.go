// Vision: PackageScope is the go test package pattern only — not a production/scoring boundary.
package gate

// PackageScope identifies which packages to run [go test], [go build], [go vet], lint, or
// deadcode against—including the test-run pass of [CoveredTest]. It is not the production
// measurement boundary; use [QualityScope] for coverpkg seeding and exclude/test-file filters.
// Zero value is invalid; use [NewPackageScope].
type PackageScope struct {
	pattern string
}

// NewPackageScope returns a scope for the given go test package pattern (e.g. "./...", "./integration/...").
func NewPackageScope(pattern string) (PackageScope, error) {
	trimmed, err := validateGoTestPackagePattern(pattern, ErrPackageScopeEmpty)
	if err != nil {
		return PackageScope{}, err
	}
	return PackageScope{pattern: trimmed}, nil
}

// Packages returns the package pattern passed to go test.
func (p PackageScope) Packages() string {
	return p.pattern
}
