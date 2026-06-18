// Vision: Quality scope inventory step: store scoped go-list package rows for explicit downstream reuse.
package harness

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const (
	QualityScopeInventoryArtifactName   = "quality-scope-package-rows.json"
	QualityScopeInventorySchemaVersion  = 1
	QualityScopeInventoryProvenanceTool = "go list -e"
	QualityScopeInventoryGoListFormat   = gatecheck.QualityScopeListFormat
)

var ErrQualityScopeInventoryFailed = gatecheck.ErrQualityScopeInventoryFailed

type qualityScopeInventoryArtifact struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Format        string                       `json:"format"`
	Packages      []qualityScopeInventoryEntry `json:"packages"`
}

type qualityScopeInventoryEntry struct {
	ImportPath      string   `json:"importPath"`
	PkgDirRootRel   string   `json:"pkgDirRootRel"`
	GoFileNames     []string `json:"goFileNames"`
	TestGoFileNames []string `json:"testGoFileNames"`
	XTestFileNames  []string `json:"xTestFileNames"`
}

func qualityScopeInventoryArtifactBytes(rows []gatecheck.MutationPackageRow) ([]byte, error) {
	entries := make([]qualityScopeInventoryEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, qualityScopeInventoryEntry{
			ImportPath:      row.ImportPath,
			PkgDirRootRel:   row.PkgDirRootRel,
			GoFileNames:     append([]string(nil), row.GoFileNames...),
			TestGoFileNames: append([]string(nil), row.TestGoFileNames...),
			XTestFileNames:  append([]string(nil), row.XTestFileNames...),
		})
	}
	return json.MarshalIndent(qualityScopeInventoryArtifact{
		SchemaVersion: QualityScopeInventorySchemaVersion,
		Format:        QualityScopeInventoryGoListFormat,
		Packages:      entries,
	}, "", "  ")
}

func (sr *StepRunner) StepQualityScopeInventory(
	ctx context.Context,
	buildTags string,
) ([]gatecheck.MutationPackageRow, error) {
	rows, err := sr.goListQualityScopePackageRows(
		ctx,
		sr.packages,
		buildTags,
		ErrQualityScopeInventoryFailed,
	)
	if err != nil {
		return nil, err
	}
	data, err := qualityScopeInventoryArtifactBytes(rows)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal inventory artifact: %w", ErrQualityScopeInventoryFailed, err)
	}
	prov := Provenance{
		StepID:   sr.stepID,
		Tool:     QualityScopeInventoryProvenanceTool,
		Packages: sr.packages,
	}
	if writeErr := sr.store.Write(sr.stepID, QualityScopeInventoryArtifactName, data, prov); writeErr != nil {
		return nil, fmt.Errorf("%w: store inventory artifact: %w", ErrQualityScopeInventoryFailed, writeErr)
	}
	return append([]gatecheck.MutationPackageRow(nil), rows...), nil
}
