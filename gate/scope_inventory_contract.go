// Vision: Public quality-scope inventory artifact contract for explicit package discovery reuse.
package gate

import (
	"encoding/json"
	"fmt"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
	"github.com/hotchkj/mage-gate/internal/harness"
)

const QualityScopeInventoryArtifactName = harness.QualityScopeInventoryArtifactName

const (
	qualityScopeInventorySchemaVersion = harness.QualityScopeInventorySchemaVersion
	qualityScopeInventoryGoListFormat  = harness.QualityScopeInventoryGoListFormat
)

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

func decodeQualityScopeInventoryArtifact(data []byte) (
	[]gatecheck.MutationPackageRow,
	error,
) {
	var artifact qualityScopeInventoryArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("%w: decode inventory artifact: %w", ErrQualityScopeInventoryInvalid, err)
	}
	if artifact.SchemaVersion != qualityScopeInventorySchemaVersion {
		return nil, fmt.Errorf(
			"%w: schema version %d",
			ErrQualityScopeInventoryInvalid,
			artifact.SchemaVersion,
		)
	}
	if artifact.Format != qualityScopeInventoryGoListFormat {
		return nil, fmt.Errorf(
			"%w: go list format mismatch",
			ErrQualityScopeInventoryInvalid,
		)
	}
	rows := make([]gatecheck.MutationPackageRow, 0, len(artifact.Packages))
	for _, row := range artifact.Packages {
		rows = append(rows, gatecheck.MutationPackageRow{
			ImportPath:      row.ImportPath,
			PkgDirRootRel:   row.PkgDirRootRel,
			GoFileNames:     append([]string(nil), row.GoFileNames...),
			TestGoFileNames: append([]string(nil), row.TestGoFileNames...),
			XTestFileNames:  append([]string(nil), row.XTestFileNames...),
		})
	}
	return rows, nil
}
