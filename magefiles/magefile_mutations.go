//go:build mage
// +build mage

// Mutation testing targets (MutationSites, MutationCoverage, and MutationKills).
package main

import (
	"context"

	qg "github.com/hotchkj/mage-gate/gate"
)

func runMutationSites() error {
	cfg, scope, _, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	runner, err := newRunner()
	if err != nil {
		return err
	}
	resolver := newResolver()
	store := newArtifactStore()
	fileOps := newFileOps()
	root := "."
	ctx := context.Background()
	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, root, scope)
	if err != nil {
		return err
	}
	mutationOpts := gremlinsMutationOptions(&cfg)
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}
	scanOut, scanErr := mr.Scan(ctx, root, scope, inv, gremlinsToolSpec(&cfg), mutationOpts...)
	if scanErr != nil {
		return scanErr
	}
	return qg.MutationSites(scanOut, mutationSitesMax(&cfg))
}

func runMutationCoverage() error {
	cfg, scope, _, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	runner, err := newRunner()
	if err != nil {
		return err
	}
	resolver := newResolver()
	store := newArtifactStore()
	fileOps := newFileOps()
	root := "."
	ctx := context.Background()
	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, root, scope)
	if err != nil {
		return err
	}
	mutationOpts := gremlinsMutationOptions(&cfg)
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}
	scanOut, scanErr := mr.Scan(ctx, root, scope, inv, gremlinsToolSpec(&cfg), mutationOpts...)
	if scanErr != nil {
		return scanErr
	}
	return qg.MutationCoverage(scanOut, mutationCoverageThreshold(&cfg))
}

// MutationSites runs the mutation-site budget step (gremlins always uses --dry-run).
func MutationSites() error {
	return runMutationSites()
}

// MutationCoverage runs gremlins dry-run and checks mutation coverage over the scan artifact.
// MinMutationCoverage(0) disables threshold enforcement when [thresholds].mutation_coverage_min is omitted or zero.
func MutationCoverage() error {
	return runMutationCoverage()
}

func runMutationKills() error {
	cfg, scope, _, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	runner, err := newRunner()
	if err != nil {
		return err
	}
	resolver := newResolver()
	store := newArtifactStore()
	fileOps := newFileOps()
	root := "."
	ctx := context.Background()
	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, root, scope)
	if err != nil {
		return err
	}

	mutationOpts := gremlinsMutationOptions(&cfg)
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}

	killOut, err := mr.Kill(ctx, root, scope, inv, gremlinsToolSpec(&cfg), mutationOpts...)
	if err != nil {
		return err
	}
	threshold := mutationKillsThreshold(&cfg)
	if killRateErr := qg.MutationKillRate(killOut, threshold); killRateErr != nil {
		return killRateErr
	}
	return nil
}

// MutationKills runs full mutation testing and validates kill rate.
// This is an on-demand step NOT wired into the Gate() precommit path.
// Usage: mage mutationKills
func MutationKills() error {
	return runMutationKills()
}
