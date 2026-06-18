//go:build mage
// +build mage

package main

import (
	"context"

	qg "github.com/hotchkj/mage-gate/gate"
)

type lintToolchainGateStep func(
	context.Context,
	qg.CommandRunner,
	qg.ToolResolver,
	qg.FileOps,
	string,
	qg.PackageScope,
	qg.LintToolchain,
) error

func runLintToolchainStep(step lintToolchainGateStep) error {
	cfg, _, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		lt, ltErr := lintToolchain(&cfg)
		if ltErr != nil {
			return ltErr
		}
		return step(
			context.Background(), runner, newResolver(), newFileOps(), repoRoot, pkgScope, lt,
		)
	})
}

func runLint() error {
	return runLintToolchainStep(qg.Lint)
}

// Lint runs the lint step.
func Lint() error {
	return runLint()
}

func runFormat() error {
	return runLintToolchainStep(qg.Format)
}

// Format runs the format step (apply formatters before lint).
func Format() error {
	return runFormat()
}

func runDeadcode() error {
	cfg, _, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		return qg.Deadcode(
			context.Background(), runner, newResolver(), newFileOps(), repoRoot, pkgScope,
			deadcodeToolSpec(&cfg), deadcodeOptions(&cfg)...,
		)
	})
}

// Deadcode runs the deadcode step.
func Deadcode() error {
	return runDeadcode()
}

func runMarkdownLint() error {
	cfg, _, _, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		return qg.MarkdownLint(
			context.Background(), runner, newResolver(), newFileOps(), repoRoot,
			markdownlintToolSpec(&cfg), markdownlintOpts(&cfg)...,
		)
	})
}

// MarkdownLint runs the markdownlint step.
func MarkdownLint() error {
	return runMarkdownLint()
}

func runCompile() error {
	_, _, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		return qg.Compile(context.Background(), runner, newFileOps(), repoRoot, pkgScope)
	})
}

// Compile runs the compile step (go build verification only; not an artifact build).
func Compile() error {
	return runCompile()
}

func runVet() error {
	_, _, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		return qg.Vet(context.Background(), runner, newFileOps(), repoRoot, pkgScope)
	})
}

// Vet runs the vet step.
func Vet() error {
	return runVet()
}

func runUnittests() error {
	cfg, scope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	if validateErr := cfg.validateIntegrationConfig(); validateErr != nil {
		return validateErr
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		ctx := context.Background()
		store := newArtifactStore()
		fileOps := newFileOps()
		inv, invErr := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, repoRoot, scope)
		if invErr != nil {
			return invErr
		}
		_, err = runCoverageTestPhase(ctx, runner, store, fileOps, &cfg, pkgScope, scope, &inv, repoRoot)
		return err
	})
}

// Unittests runs the coverage-bearing primary `go test` pass ([unittests] + [quality_scope]).
func Unittests() error {
	return runUnittests()
}

func runIntegrationtests() error {
	cfg, _, _, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	if validateErr := cfg.validateIntegrationConfig(); validateErr != nil {
		return validateErr
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		return runIntegrationPass(
			context.Background(), runner, newArtifactStore(), newFileOps(), repoRoot, &cfg, true,
		)
	})
}

// Integrationtests runs gate.toml [integrationtests] only (no merged coverage).
func Integrationtests() error {
	return runIntegrationtests()
}

// runCoverageTestPhase runs the coverage-producing primary `go test` pass.
func runQualityScopeInventoryPhase(
	ctx context.Context,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	root string,
	qualityScope qg.QualityScope,
) (qg.QualityScopeInventoryOutput, error) {
	return qg.QualityScopeInventory(ctx, runner, store, fileOps, root, qualityScope)
}

func runCoverageTestPhase(
	ctx context.Context,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	cfg *config,
	pkgScope qg.PackageScope,
	qualityScope qg.QualityScope,
	inventory *qg.QualityScopeInventoryOutput,
	root string,
) (qg.CoveredTestOutput, error) {
	return qg.CoveredTest(
		ctx, runner, store, fileOps, root, pkgScope, qualityScope, *inventory,
		primaryPassOpts(cfg)...,
	)
}

// runDuration, runCoverage, and runCrap are standalone mage entrypoints. Each process has
// its own ArtifactStore, so typed tokens cannot be passed from another mage invocation;
// these targets run prerequisite steps (CoveredTest, and Coverage before Crap) in-process.
func runDuration() error {
	cfg, scope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		ctx := context.Background()
		store := newArtifactStore()
		fileOps := newFileOps()
		inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, repoRoot, scope)
		if err != nil {
			return err
		}
		unitCov, err := runCoverageTestPhase(ctx, runner, store, fileOps, &cfg, pkgScope, scope, &inv, repoRoot)
		if err != nil {
			return err
		}
		testRun, trErr := unitCov.TestRun()
		if trErr != nil {
			return trErr
		}
		return qg.Duration(
			ctx, runner, store, fileOps, repoRoot, testRun, durationMax(&cfg),
		)
	})
}

// Duration runs the duration step.
func Duration() error {
	return runDuration()
}

func runCoverage() error {
	cfg, scope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		ctx := context.Background()
		store := newArtifactStore()
		fileOps := newFileOps()
		inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, repoRoot, scope)
		if err != nil {
			return err
		}
		unitCov, err := runCoverageTestPhase(ctx, runner, store, fileOps, &cfg, pkgScope, scope, &inv, repoRoot)
		if err != nil {
			return err
		}
		_, err = qg.Coverage(ctx, runner, store, fileOps, repoRoot, unitCov, coverageMin(&cfg))
		return err
	})
}

// Coverage runs the coverage step.
func Coverage() error {
	return runCoverage()
}

func runCrap() error {
	cfg, scope, pkgScope, err := loadConfigAndScope()
	if err != nil {
		return err
	}
	return withRepositoryRootNoResult(func(repoRoot string) error {
		runner, runErr := newRunner()
		if runErr != nil {
			return runErr
		}
		ctx := context.Background()
		store := newArtifactStore()
		fileOps := newFileOps()
		resolver := newResolver()
		inv, err := runQualityScopeInventoryPhase(ctx, runner, store, fileOps, repoRoot, scope)
		if err != nil {
			return err
		}
		unitCov, err := runCoverageTestPhase(ctx, runner, store, fileOps, &cfg, pkgScope, scope, &inv, repoRoot)
		if err != nil {
			return err
		}
		covOut, err := qg.Coverage(ctx, runner, store, fileOps, repoRoot, unitCov, coverageMin(&cfg))
		if err != nil {
			return err
		}
		return qg.Crap(
			ctx, runner, resolver, store, fileOps, repoRoot, covOut, inv,
			crapMax(&cfg), gocycloToolSpec(&cfg), crapOptions(&cfg)...,
		)
	})
}

// Crap runs the CRAP step.
func Crap() error {
	return runCrap()
}
