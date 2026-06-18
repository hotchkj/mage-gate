// Vision: FakeRunner response builders parameterized per command key—compose stdout/stderr/exit without bespoke types.
// Generic helpers (NoopCommand, Fail, FailWith) are re-exported from cmdtest.
package gatetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// CommandFunc is cmdtest.CommandFunc — re-exported for consumers that only import gatetest.
type CommandFunc = cmdtest.CommandFunc

// FileOpsWriter is the write-only subset of FileOps accepted by response
// factories that produce side-effect files (coverprofile, mutation JSON).
type FileOpsWriter interface {
	MkdirAll(path string, perm fs.FileMode) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
}

const (
	coverFilePerm       = 0o600
	coverDirPerm        = 0o700
	fullCoveragePercent = 100
	moduleJSONFormat    = "__module_json__"
)

var (
	errGoListFakeMissingPackageArg = fmt.Errorf("go list fake requires an explicit package argument")
	errGoListFakeMultiplePackages  = fmt.Errorf("go list fake expects one package argument")
	errGoListFakeUnsupportedFormat = fmt.Errorf("go list fake unsupported format")
	// ErrFakeCoveragePercentOutOfRange reports a disallowed percent for [GoTestPassWithCoverage] fakes.
	ErrFakeCoveragePercentOutOfRange = errors.New("gatetest: fake coverage percent must be between 0 and 100 inclusive")
	// ErrGremlinsReportPathEscape reports a gremlins -o path that escapes the configured anchor (root or cmd.Dir).
	ErrGremlinsReportPathEscape = errors.New("gatetest: gremlins report path escapes root")
)

func errGremlinsPathEscapef(anchor, resolved string) error {
	return fmt.Errorf("%w: %q escapes anchor %q", ErrGremlinsReportPathEscape, resolved, anchor)
}

// PackageListInfo is directory and optional [go list] TestGoFiles / XTestGoFiles basenames
// (used by [GoList] and the mutation TSV line format).
type PackageListInfo struct {
	Dir     string
	Test    []string
	XTest   []string
	GoFiles []string // non-test .go basenames for mutation inventory (optional in fakes)
}

// DirOnly returns a [PackageListInfo] with no test go files.
func DirOnly(d string) PackageListInfo { return PackageListInfo{Dir: d} }

// NoopCommand always succeeds and writes nothing.
var NoopCommand = cmdtest.NoopCommand //nolint:gochecknoglobals // re-export

// Fail returns a CommandFunc that always returns the given error.
func Fail(err error) CommandFunc { return cmdtest.Fail(err) }

// FailWith returns a CommandFunc that writes output to stdout before returning the error.
func FailWith(err error, output string) CommandFunc { return cmdtest.FailWith(err, output) }

// GoTestPass returns a CommandFunc that emits realistic go test -json events to
// stdout and writes a minimal coverprofile if the -coverprofile flag is present.
// The emitted sequence mirrors real go test output: a run event, a test-level pass event,
// then a package-level pass event. Duration checks read the test-level completion
// event and ignore package-level wall-clock.
func coverprofilePathFromArgs(args []string) string {
	for i, arg := range args {
		if strings.HasPrefix(arg, "-coverprofile=") {
			return strings.TrimPrefix(arg, "-coverprofile=")
		}
		if arg == "-coverprofile" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// CoverprofilePathFromArgs returns the path from -coverprofile=path or -coverprofile path,
// or "" if absent (same parsing as [GoTestPass], [GoTestPassWithCoverage], and feature-contract fakes).
func CoverprofilePathFromArgs(args []string) string {
	return coverprofilePathFromArgs(args)
}

// minimalCoverProfileBody is a tiny coverage.out with one statement line so downstream
// steps (coverage threshold with quality excludes, CRAP materialization) see non-empty
// profile data after filtering — not just a mode header.
const minimalCoverProfileBody = "mode: set\nexample.com/mod/pkg/file.go:1.1,2.2 3 1\n"

func writeMinimalCoverprofile(fileOps FileOpsWriter, coverProfilePath string) error {
	coverProfilePath = fsnorm.Canonical(coverProfilePath)
	if mkErr := fileOps.MkdirAll(fsnorm.Dir(coverProfilePath), coverDirPerm); mkErr != nil {
		return mkErr
	}
	return fileOps.WriteFile(coverProfilePath, []byte(minimalCoverProfileBody), coverFilePerm)
}

// WriteMinimalCoverprofile writes the same minimal cover profile as [GoTestPass] for fake commands.
func WriteMinimalCoverprofile(fileOps FileOpsWriter, coverProfilePath string) error {
	return writeMinimalCoverprofile(fileOps, coverProfilePath)
}

func writeGoTestPassEvents(stdout io.Writer, packageName string) error {
	run := fmt.Sprintf(`{"Action":"run","Package":%q,"Test":"TestPass"}`, packageName) + "\n"
	testPass := fmt.Sprintf(
		`{"Action":"pass","Package":%q,"Test":"TestPass","Elapsed":0.01}`, packageName,
	) + "\n"
	pkgPass := fmt.Sprintf(
		`{"Action":"pass","Package":%q,"Elapsed":0.01}`, packageName,
	) + "\n"
	if _, err := io.WriteString(stdout, run); err != nil {
		return err
	}
	if _, err := io.WriteString(stdout, testPass); err != nil {
		return err
	}
	if _, err := io.WriteString(stdout, pkgPass); err != nil {
		return err
	}
	return nil
}

// GoTestPassWithCoverage returns a CommandFunc that emits realistic go test -json events to
// stdout and writes a coverprofile with the specified coverage percentage.
// The coverage percentage is calculated as (coveredStatements / totalStatements) * 100.
// Uses 100 total statements; covered = (pct/100) * 100.
func GoTestPassWithCoverage(fileOps FileOpsWriter, packageName string, coveragePercent int) CommandFunc {
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		if coveragePercent < 0 || coveragePercent > fullCoveragePercent {
			return fmt.Errorf("%w: got %d", ErrFakeCoveragePercentOutOfRange, coveragePercent)
		}
		if err := writeGoTestPassEvents(stdout, packageName); err != nil {
			return err
		}

		coverProfilePath := coverprofilePathFromArgs(cmd.Args())
		if coverProfilePath == "" {
			return nil
		}
		return writeCoverprofileWithCoverage(fileOps, coverProfilePath, coveragePercent)
	}
}

// writeCoverprofileWithCoverage writes a coverage.out file with the specified statement coverage percentage.
func writeCoverprofileWithCoverage(fileOps FileOpsWriter, coverProfilePath string, coveragePercent int) error {
	coverProfilePath = fsnorm.Canonical(coverProfilePath)
	if mkErr := fileOps.MkdirAll(fsnorm.Dir(coverProfilePath), coverDirPerm); mkErr != nil {
		return mkErr
	}

	const totalStatements = 100
	coveredStatements := (coveragePercent * totalStatements) / fullCoveragePercent
	uncoveredStatements := totalStatements - coveredStatements

	var profile strings.Builder
	profile.WriteString("mode: set\n")
	if coveredStatements > 0 {
		fmt.Fprintf(&profile, "example.com/mod/pkg/file.go:1.1,2.2 %d 1\n", coveredStatements)
	}
	if uncoveredStatements > 0 {
		fmt.Fprintf(&profile, "example.com/mod/pkg/file.go:3.1,4.2 %d 0\n", uncoveredStatements)
	}

	return fileOps.WriteFile(coverProfilePath, []byte(profile.String()), coverFilePerm)
}

func GoTestPass(fileOps FileOpsWriter, packageName string) CommandFunc {
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		if err := writeGoTestPassEvents(stdout, packageName); err != nil {
			return err
		}

		coverProfilePath := coverprofilePathFromArgs(cmd.Args())
		if coverProfilePath == "" {
			return nil
		}
		return writeMinimalCoverprofile(fileOps, coverProfilePath)
	}
}

// GoToolCover returns a CommandFunc that outputs the total coverage
// percentage in go tool cover -func format.
func GoToolCover(totalPercent float64) CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
		_, err := fmt.Fprintf(stdout, "total:\t(statements)\t%.1f%%\n", totalPercent)
		return err
	}
}

// GoToolCoverFunc returns a CommandFunc that outputs per-function
// coverage lines followed by a total line in go tool cover -func format.
// The funcs map keys are "path/to/file.go:line:\tFuncName" identifiers;
// values are coverage percentages.
func GoToolCoverFunc(funcs map[string]float64, totalPercent float64) CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
		keys := make([]string, 0, len(funcs))
		for id := range funcs {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			if _, err := fmt.Fprintf(stdout, "%s\t\t%.1f%%\n", id, funcs[id]); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(stdout, "total:\t(statements)\t%.1f%%\n", totalPercent)
		return err
	}
}

// GoList returns a CommandFunc that handles go list queries. It supports
// module queries (-m -f format) and package queries (-f format).
//
// For module queries:
//   - {{.Path}} returns modulePath
//   - {{.Dir}} returns moduleDir
//   - -json returns one module object with Path and Dir fields
//
// For package queries:
//   - {{.ImportPath}} returns each package import path
//   - {{.Dir}} returns each package directory
//
// The packages map keys are import paths; values are [PackageListInfo] entries.
func GoList(modulePath, moduleDir string, packages map[string]PackageListInfo) CommandFunc {
	return func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
		args := cmd.Args()
		isMod, format := parseListArgs(args)
		if isMod {
			return writeModuleOutput(stdout, format, modulePath, moduleDir)
		}
		return writePackageOutput(stdout, format, moduleDir, packages, args)
	}
}

func parseListArgs(args []string) (isMod bool, format string) {
	for idx := 0; idx < len(args); idx++ {
		switch args[idx] {
		case "-m":
			isMod = true
		case "-json":
			format = moduleJSONFormat
		case "-f":
			if idx+1 < len(args) {
				format = args[idx+1]
				idx++
			}
		}
	}
	return isMod, format
}

func writeModuleOutput(writer io.Writer, format, modulePath, moduleDir string) error {
	switch format {
	case "{{.Path}}":
		_, err := fmt.Fprintln(writer, modulePath)
		return err
	case "{{.Dir}}":
		_, err := fmt.Fprintln(writer, moduleDir)
		return err
	case moduleJSONFormat:
		return json.NewEncoder(writer).Encode(struct {
			Path string
			Dir  string
		}{
			Path: modulePath,
			Dir:  moduleDir,
		})
	}
	return fmt.Errorf("%w: %q", errGoListFakeUnsupportedFormat, format)
}

// goListPackageArg returns the explicit package argument in a go list command (e.g. "./...").
func goListPackageArg(args []string) (string, error) {
	var pkgArg string
	for argIndex := 0; argIndex < len(args); argIndex++ {
		arg := args[argIndex]
		if arg == "" || arg == "list" {
			continue
		}
		switch {
		case arg == "-f" || arg == "-tags":
			argIndex++
			continue
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			if pkgArg != "" {
				return "", fmt.Errorf("%w: got %q and %q", errGoListFakeMultiplePackages, pkgArg, arg)
			}
			pkgArg = arg
		}
	}
	if pkgArg == "" {
		return "", errGoListFakeMissingPackageArg
	}
	return pkgArg, nil
}

func filterPackageKeysForListPattern(
	moduleDir string,
	packages map[string]PackageListInfo,
	listPattern string,
) []string {
	keys := sortedPackageKeys(packages)
	listPattern = fsnorm.Canonical(listPattern)
	if listPattern == "..." {
		return keys
	}
	if strings.HasSuffix(listPattern, "/...") {
		return filterPackageKeysByPrefix(keys, moduleDir, packages, listPattern)
	}
	pkgPath := strings.TrimPrefix(listPattern, "./")
	pkgPath = fsnorm.Canonical(pkgPath)
	if pkgPath == "" {
		return nil
	}
	return filterPackageKeysByExactPath(keys, moduleDir, packages, pkgPath)
}

func sortedPackageKeys(packages map[string]PackageListInfo) []string {
	keys := make([]string, 0, len(packages))
	for k := range packages {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func filterPackageKeysByPrefix(
	keys []string,
	moduleDir string,
	packages map[string]PackageListInfo,
	listPattern string,
) []string {
	seg := strings.TrimPrefix(listPattern, "./")
	seg = strings.TrimSuffix(seg, "/...")
	seg = fsnorm.Canonical(seg)
	if seg == "" {
		return nil
	}
	var out []string
	for _, importKey := range keys {
		if packageDirMatchesListPrefix(moduleDir, packages[importKey].Dir, seg) {
			out = append(out, importKey)
		}
	}
	return out
}

func filterPackageKeysByExactPath(
	keys []string,
	moduleDir string,
	packages map[string]PackageListInfo,
	pkgPath string,
) []string {
	var out []string
	for _, importKey := range keys {
		rel, ok := packageDirRelToModule(moduleDir, packages[importKey].Dir)
		if ok && rel == pkgPath {
			out = append(out, importKey)
		}
	}
	return out
}

func packageDirMatchesListPrefix(moduleDir, packageDir, prefix string) bool {
	rel, ok := packageDirRelToModule(moduleDir, packageDir)
	return ok && (rel == prefix || strings.HasPrefix(rel, prefix+"/"))
}

func packageDirRelToModule(moduleDir, packageDir string) (string, bool) {
	moduleDir = fsnorm.Canonical(moduleDir)
	packageDir = fsnorm.Canonical(packageDir)
	if moduleDir == "" {
		return packageDir, true
	}
	rel, err := fsnorm.Rel(moduleDir, packageDir)
	if err == nil {
		return rel, true
	}
	return "", false
}

func writePackageOutput(
	writer io.Writer,
	format, moduleDir string,
	packages map[string]PackageListInfo,
	goListArgs []string,
) error {
	pkgArg, err := goListPackageArg(goListArgs)
	if err != nil {
		return err
	}
	keys := filterPackageKeysForListPattern(moduleDir, packages, pkgArg)
	for _, importPath := range keys {
		pkg := packages[importPath]
		switch format {
		case gatecheck.QualityScopeListFormat:
			joinedTest := strings.Join(pkg.Test, ";")
			joinedX := strings.Join(pkg.XTest, ";")
			joinedGo := strings.Join(pkg.GoFiles, ";")
			modCol := moduleDir
			if _, err := fmt.Fprintf(
				writer, "%s\t%s\t%s\t%s\t%s\t%s\n", importPath, pkg.Dir, joinedTest, joinedX, modCol, joinedGo,
			); err != nil {
				return err
			}
		case "{{.ImportPath}}":
			if _, err := fmt.Fprintln(writer, importPath); err != nil {
				return err
			}
		case "{{.Dir}}":
			if _, err := fmt.Fprintln(writer, pkg.Dir); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %q", errGoListFakeUnsupportedFormat, format)
		}
	}
	return nil
}

// Gocyclo returns a CommandFunc that outputs cyclomatic complexity
// scores in gocyclo format: "N pkg Func file:line:col".
// The scores map keys are "funcName" identifiers; values are complexity scores.
// Each entry produces a line with a synthetic file location.
func Gocyclo(scores map[string]int) CommandFunc {
	return func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
		for funcID, complexity := range scores {
			if _, err := fmt.Fprintf(stdout, "%d pkg %s file.go:1:1\n", complexity, funcID); err != nil {
				return err
			}
		}
		return nil
	}
}

// Gremlins returns a CommandFunc that writes a mutation test JSON report
// to the path specified by the -o flag (argv realm: -o path or -o=path), using the same
// canonical logical paths [MemoryFileOps] and harness FileOps read back—never host filepath.Abs
// joins for ordinary relative outputs.
func Gremlins(fileOps FileOpsWriter, root string, report []byte) CommandFunc {
	return func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
		outPath := gremlinsOutputPathFromArgs(cmd.Args())
		if strings.TrimSpace(outPath) == "" {
			return nil
		}
		effectiveRoot := root
		if root == "" {
			effectiveRoot = cmd.Dir()
		}
		fullPath, err := gremlinsReportLogicalDestination(effectiveRoot, outPath)
		if err != nil {
			return err
		}
		if fullPath == "" {
			return nil
		}
		if mkErr := fileOps.MkdirAll(fsnorm.Dir(fullPath), coverDirPerm); mkErr != nil {
			return mkErr
		}
		return fileOps.WriteFile(fullPath, report, coverFilePerm)
	}
}

func gremlinsOutputPathFromArgs(args []string) string {
	for i, arg := range args {
		if strings.HasPrefix(arg, "-o=") {
			return strings.TrimPrefix(arg, "-o=")
		}
		if arg == "-o" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func gremlinsArgvLexicallyAbsolute(raw, canon string) bool {
	if filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" {
		return true
	}
	if canon == "" {
		return false
	}
	if strings.HasPrefix(canon, "/") {
		return true
	}
	if gatecheck.IsWindowsDriveLexicalCanon(canon) {
		return true
	}
	// UNC and similar //server/share/... after canonicalization.
	return strings.HasPrefix(canon, "//")
}

func gremlinsRelContainsDotDotSegment(rel string) bool {
	if rel == "" || rel == "." {
		return false
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func gremlinsContainedUnderAnchor(anchorCanon, resolvedCanon string) error {
	if anchorCanon == "" || anchorCanon == "." {
		r := resolvedCanon
		if gremlinsRelContainsDotDotSegment(r) {
			return errGremlinsPathEscapef(anchorCanon, resolvedCanon)
		}
		return nil
	}
	rel, err := fsnorm.Rel(anchorCanon, resolvedCanon)
	if err != nil {
		return errGremlinsPathEscapef(anchorCanon, resolvedCanon)
	}
	if gremlinsRelContainsDotDotSegment(rel) {
		return errGremlinsPathEscapef(anchorCanon, resolvedCanon)
	}
	return nil
}

func gremlinsReportLogicalDestination(anchorRoot, outArgv string) (string, error) {
	outArgv = strings.TrimSpace(outArgv)
	if outArgv == "" {
		return "", nil
	}
	canonArg := fsnorm.Canonical(outArgv)
	anchorCanon := fsnorm.Canonical(anchorRoot)
	if anchorCanon == "" {
		anchorCanon = "."
	}

	if !gremlinsArgvLexicallyAbsolute(outArgv, canonArg) {
		if gremlinsRelContainsDotDotSegment(canonArg) {
			return "", errGremlinsPathEscapef(anchorCanon, canonArg)
		}
		return canonArg, nil
	}
	if err := gremlinsContainedUnderAnchor(anchorCanon, canonArg); err != nil {
		return "", err
	}
	if anchorCanon == "." {
		return "", errGremlinsPathEscapef(anchorCanon, canonArg)
	}
	rel, err := fsnorm.Rel(anchorCanon, canonArg)
	if err != nil {
		return "", errGremlinsPathEscapef(anchorCanon, canonArg)
	}
	if gremlinsRelContainsDotDotSegment(rel) {
		return "", errGremlinsPathEscapef(anchorCanon, canonArg)
	}
	return rel, nil
}
