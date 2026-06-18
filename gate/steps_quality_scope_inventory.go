package gate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	"github.com/hotchkj/mage-gate/internal/harness"
)

func inventoryRoot(root string) string {
	return fsnorm.Canonical(gateRoot(root))
}

// requireQualityScopeInventory validates the opaque inventory handle, store/root match,
// scope fingerprint match, and artifact-chain presence.
func requireQualityScopeInventory(
	inventory *QualityScopeInventoryOutput,
	store *ArtifactStore,
	root string,
	qualityScope QualityScope,
) error {
	if err := validateInventoryTokenShape(inventory); err != nil {
		return err
	}
	if err := validateInventoryTokenMatch(inventory, store, root); err != nil {
		return err
	}
	if err := validateInventoryScopeFingerprint(inventory, qualityScope); err != nil {
		return err
	}
	return requireUpstreamArtifact(store, inventory.stepID, QualityScopeInventoryArtifactName)
}

func validateInventoryScopeFingerprint(inventory *QualityScopeInventoryOutput, qs QualityScope) error {
	if inventory.scopeFingerprint != qualityScopeFingerprint(qs) {
		return fmt.Errorf("%w: scope", ErrQualityScopeInventoryMismatch)
	}
	return nil
}

func validateInventoryTokenShape(inventory *QualityScopeInventoryOutput) error {
	if inventory == nil {
		return fmt.Errorf("%w: QualityScopeInventoryOutput is nil", ErrQualityScopeInventoryInvalid)
	}
	if inventory.store == nil || inventory.stepID == "" || len(inventory.rows) == 0 {
		return fmt.Errorf("%w: incomplete token", ErrQualityScopeInventoryInvalid)
	}
	if inventory.schema != qualityScopeInventorySchemaVersion || inventory.format != qualityScopeInventoryGoListFormat {
		return fmt.Errorf("%w: artifact contract mismatch", ErrQualityScopeInventoryInvalid)
	}
	return nil
}

func validateInventoryTokenMatch(
	inventory *QualityScopeInventoryOutput,
	store *ArtifactStore,
	root string,
) error {
	if inventory.store != store {
		return fmt.Errorf("%w: store", ErrQualityScopeInventoryMismatch)
	}
	if inventory.root != inventoryRoot(root) {
		return fmt.Errorf("%w: root", ErrQualityScopeInventoryMismatch)
	}
	return nil
}

func inventoryRowsForConsumer(inventory *QualityScopeInventoryOutput) []gatecheck.MutationPackageRow {
	return append([]gatecheck.MutationPackageRow(nil), inventory.rows...)
}

func qualityScopeCommandScope(
	qs QualityScope,
	inventory *QualityScopeInventoryOutput,
) *gatecheck.QualityScopeCommandScope {
	var rows []gatecheck.MutationPackageRow
	if inventory != nil {
		rows = inventoryRowsForConsumer(inventory)
	}
	commandScope := gatecheck.NewQualityScopeCommandScope(
		rows,
		nil,
		joinSorted(qualityScopeExcludeSegments(qs)),
		qs.TestFilePatterns(),
		qs.Tags(),
	)
	return &commandScope
}

func rejectBuildTagArgs(step string, args []string) error {
	for _, arg := range args {
		switch {
		case arg == "-tags" || arg == "--tags":
			return newValidationError(step, "build tags belong in QualityScope.Tags()", ErrInvalidOption)
		case strings.HasPrefix(arg, "-tags=") || strings.HasPrefix(arg, "--tags="):
			return newValidationError(step, "build tags belong in QualityScope.Tags()", ErrInvalidOption)
		}
	}
	return nil
}

// QualityScopeInventory runs go list for [QualityScope] and stores the canonical package inventory.
//
//nolint:gocritic // Opaque value token
func QualityScopeInventory(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	qualityScope QualityScope,
) (out QualityScopeInventoryOutput, err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return QualityScopeInventoryOutput{}, rootErr
	}
	emitStepStart(runner, "Quality Scope Inventory", "")
	if checkErr := validateQualityScope(qualityScope); checkErr != nil {
		return QualityScopeInventoryOutput{}, checkErr
	}
	if checkErr := requireStoreDeps(runner, fileOps, store); checkErr != nil {
		return QualityScopeInventoryOutput{}, checkErr
	}
	id := nextID("qualityscopeinventory")
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", qualityScopePackages(qualityScope), runner, fileOps, store, id,
	)
	if err != nil {
		return QualityScopeInventoryOutput{}, fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("qualityscopeinventory", runner, harn.Cleanup())) }()
	rows, stepErr := harn.StepQualityScopeInventory(ctx, joinSorted(qualityScopeTags(qualityScope)))
	if stepErr != nil {
		return QualityScopeInventoryOutput{}, wrapStepError("qualityscopeinventory", runner, stepErr)
	}
	return QualityScopeInventoryOutput{
		store:            store,
		stepID:           id,
		root:             inventoryRoot(root),
		rows:             append([]gatecheck.MutationPackageRow(nil), rows...),
		schema:           qualityScopeInventorySchemaVersion,
		format:           qualityScopeInventoryGoListFormat,
		scopeFingerprint: qualityScopeFingerprint(qualityScope),
	}, nil
}
