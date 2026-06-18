// Vision: Composition-root mutation operation surface: one runner holds tool/store deps for Scan vs Kill.
package gate

import (
	"context"
	"fmt"
)

// MutationRunner runs gremlins dry-run (scan) and full mutation (kill) using the same
// [CommandRunner], [ToolResolver], [ArtifactStore], and [FileOps] as other gate steps.
type MutationRunner interface {
	Scan(
		ctx context.Context,
		root string,
		qualityScope QualityScope,
		inventory QualityScopeInventoryOutput,
		gremlinsSpec GremlinsToolValue,
		opts ...MutationOption,
	) (MutationScanOutput, error)
	Kill(
		ctx context.Context,
		root string,
		qualityScope QualityScope,
		inventory QualityScopeInventoryOutput,
		gremlinsSpec GremlinsToolValue,
		opts ...MutationOption,
	) (MutationKillsOutput, error)
}

type productionMutationRunner struct {
	runner   CommandRunner
	resolver ToolResolver
	store    *ArtifactStore
	fileOps  FileOps
}

// NewMutationRunner returns a [MutationRunner] for mage or other roots that assemble
// the same dependencies as [MutationKills] and scan composition paths. Nil dependencies are
// rejected at construction (see [ErrNilDependency]).
// [Kill] returns [MutationKillsOutput] with parsed metrics and the run’s display mode for
// [MutationKillRate]; use [MutationKillRate] to enforce a minimum kill rate after the run.
func NewMutationRunner(
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
) (MutationRunner, error) {
	if runner == nil {
		return nil, fmt.Errorf("%w: runner", ErrNilDependency)
	}
	if resolver == nil {
		return nil, fmt.Errorf("%w: resolver", ErrNilDependency)
	}
	if store == nil {
		return nil, fmt.Errorf("%w: store", ErrNilDependency)
	}
	if fileOps == nil {
		return nil, fmt.Errorf("%w: fileOps", ErrNilDependency)
	}
	return &productionMutationRunner{
		runner:   runner,
		resolver: resolver,
		store:    store,
		fileOps:  fileOps,
	}, nil
}

//nolint:gocritic // Opaque value token keeps the public MutationRunner API consistent.
func (m *productionMutationRunner) Scan(
	ctx context.Context,
	root string,
	qualityScope QualityScope,
	inventory QualityScopeInventoryOutput,
	gremlinsSpec GremlinsToolValue,
	opts ...MutationOption,
) (MutationScanOutput, error) {
	if err := validateRoot(root); err != nil {
		return MutationScanOutput{}, err
	}
	if err := mutationValidateBeforeScan(
		qualityScope, gremlinsSpec, m.runner, m.resolver, m.fileOps, m.store,
	); err != nil {
		return MutationScanOutput{}, err
	}
	if err := requireQualityScopeInventory(&inventory, m.store, root, qualityScope); err != nil {
		return MutationScanOutput{}, err
	}
	return runMutationScan(
		ctx, m.runner, m.resolver, m.store, m.fileOps, root, qualityScope, &inventory,
		mutationRunnerScanMaxSites, scanOpMutationscan, gremlinsSpec, opts...,
	)
}

//nolint:gocritic // Opaque value token keeps the public MutationRunner API consistent.
func (m *productionMutationRunner) Kill(
	ctx context.Context,
	root string,
	qualityScope QualityScope,
	inventory QualityScopeInventoryOutput,
	gremlinsSpec GremlinsToolValue,
	opts ...MutationOption,
) (MutationKillsOutput, error) {
	if err := validateRoot(root); err != nil {
		return MutationKillsOutput{}, err
	}
	cfg := defaultMutationConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := rejectBuildTagArgs("mutationkills", cfg.mutationArgs); err != nil {
		return MutationKillsOutput{}, err
	}
	// Harness min kill rate 0: threshold checks are composed via [MutationKillRate] instead.
	if err := mutationValidateBeforeHarnessForKills(
		qualityScope, MinKillRate(0), gremlinsSpec, m.runner, m.resolver, m.fileOps, m.store,
	); err != nil {
		return MutationKillsOutput{}, err
	}
	if err := requireQualityScopeInventory(&inventory, m.store, root, qualityScope); err != nil {
		return MutationKillsOutput{}, err
	}
	return mutationKillsHarness(
		ctx, m.runner, m.resolver, m.store, m.fileOps, root, qualityScope, &inventory,
		MinKillRate(0), gremlinsSpec, cfg.mutationArgs,
	)
}
