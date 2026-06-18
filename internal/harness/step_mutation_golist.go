// Vision: shared go list and root-relative helpers for [QualityScope] measurement boundaries.
package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const errFmtGoListStep = "%w: go list: %w"

// packageDirModuleRelative is fsnorm.Rel(moduleRoot, pkgDir) for gremlins exclude path roots.
func packageDirModuleRelative(moduleRoot, pkgDir string) (string, error) {
	if strings.TrimSpace(moduleRoot) == "" {
		return "", nil
	}
	modPath := fsnorm.Canonical(moduleRoot)
	pd := fsnorm.Canonical(pkgDir)
	if r, err := fsnorm.Rel(modPath, pd); err == nil {
		if fileopspath.IsPathEscape(r) {
			return "", fmt.Errorf("%w: package dir %q outside module %q", fileopspath.ErrPathTraversal, pd, modPath)
		}
		return r, nil
	} else {
		return "", fmt.Errorf("%w: package dir %q relative to module %q", err, pd, modPath)
	}
}

// mutationListLineFields holds one parsed line from [goListQualityScopePackageRows] list format.
type mutationListLineFields struct {
	imp, pkgDir, testCol, xtestCol, modCol, goCol string
}

func parseGoListMutationLine(line string) (mutationListLineFields, error) {
	parts := strings.Split(line, "\t")
	switch {
	case len(parts) == 6:
		return mutationListLineFields{
			imp:      strings.TrimSpace(parts[0]),
			pkgDir:   strings.TrimSpace(parts[1]),
			testCol:  parts[2],
			xtestCol: parts[3],
			modCol:   parts[4],
			goCol:    parts[5],
		}, nil
	default:
		return mutationListLineFields{}, fmt.Errorf("%w: %q", errGoListMutationLineInvalid, line)
	}
}

func pkgDirRootRelForMutationRow(absRoot, modCol, pd string) (string, error) {
	if modRoot := fsnorm.Canonical(modCol); modRoot != "" {
		return packageDirModuleRelative(modRoot, pd)
	}
	rel, relErr := fsnorm.Rel(absRoot, pd)
	if relErr != nil {
		return "", fmt.Errorf("%w: package dir %q relative to root %q", relErr, pd, absRoot)
	}
	if fileopspath.IsPathEscape(rel) {
		return "", fmt.Errorf("%w: package dir %q outside root %q", fileopspath.ErrPathTraversal, pd, absRoot)
	}
	return rel, nil
}

func (sr *StepRunner) goListQualityScopePackageRows(
	ctx context.Context,
	coverpkgPackages string,
	buildTags string,
	stepErr error,
) ([]gatecheck.MutationPackageRow, error) {
	pkgs := resolvePackages(coverpkgPackages)
	args := []string{"list", "-e"}
	args = append(args, buildTagsFlag(buildTags)...)
	args = append(args, "-f", gatecheck.QualityScopeListFormat, pkgs)
	result, err := sr.runCommand(ctx, sr.root, "go", args...)
	if err != nil {
		return nil, fmt.Errorf(errFmtGoListStep, stepErr, err)
	}
	lines := goListOutputLines(result.Stdout)
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: go list returned no packages", stepErr)
	}
	absRoot, err := filepath.Abs(sr.root)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve module root: %w", stepErr, err)
	}
	absRoot = fsnorm.Canonical(absRoot)
	rows := make([]gatecheck.MutationPackageRow, 0, len(lines))
	for _, line := range lines {
		lineFields, perr := parseGoListMutationLine(line)
		if perr != nil {
			return nil, fmt.Errorf(errFmtGoListStep, stepErr, perr)
		}
		pd := fsnorm.Canonical(lineFields.pkgDir)
		rel, relErr := pkgDirRootRelForMutationRow(absRoot, lineFields.modCol, pd)
		if relErr != nil {
			return nil, fmt.Errorf(errFmtGoListStep, stepErr, relErr)
		}
		rel = fsnorm.Canonical(rel)
		rows = append(rows, gatecheck.MutationPackageRow{
			ImportPath:      lineFields.imp,
			PkgDirRootRel:   rel,
			GoFileNames:     splitSemicolonList(lineFields.goCol),
			TestGoFileNames: splitSemicolonList(lineFields.testCol),
			XTestFileNames:  splitSemicolonList(lineFields.xtestCol),
		})
	}
	return rows, nil
}

func goListOutputLines(value string) []string {
	lines := strings.Split(strings.TrimRight(value, "\r\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitSemicolonList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
