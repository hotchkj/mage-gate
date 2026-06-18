# Mage Gate

A composable quality gate library for Go projects. Compose quality gate stages with dependency safety — the compiler prevents invalid wiring.

## Installation

```bash
go get github.com/hotchkj/mage-gate/gate
```

## Quick Start

Construct dependencies individually: a display runner (wraps output mode), an artifact store (shared by stages that exchange tokens), file ops, and a root path. Each stage function takes exactly the dependencies it needs.

Optional: pass package patterns and `QualityScope` options from your own configuration (flags, environment variables, TOML in your entrypoint, or literals):

```go
qs, err := qg.NewQualityScope("./...",
	qg.Exclude("vendor", "testdata"),
	qg.TestFilePatterns("*_test.go"),
)
if err != nil {
	return err
}
```

This repository's `magefiles/` package reads `gate.toml` as a local convenience; the `gate` library does not include a TOML loader. The `[quality_scope]` table holds the same package pattern and options you would pass to `NewQualityScope` in code.

**Coverage %:** the number compared to your threshold (and the one you should act on) is the total from `go tool cover` on the **quality-scoped** profile: `CoveredTest` passes `-coverpkg` built from the scope with excludes removed, and `Coverage` may filter the same profile. A plain `go test ./... -coverprofile=…` *without* that scoped list reports a different aggregate, often lower when optional packages (BDD, harness) appear as 0% in the merge.

```go
//go:build mage

package main

import (
	"context"
	"os"

	qg "github.com/hotchkj/mage-gate/gate"
)

func Gate() error {
	ctx := context.Background()
	runner, err := qg.NewDisplayRunner(qg.NewProductionRunner(), qg.OutputModeAgent, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	store := qg.NewArtifactStore()
	fileOps := qg.NewProductionFileOps()
	root := "."

	qs, err := qg.NewQualityScope("./...", qg.Exclude("features"), qg.TestFilePatterns("*_test.go"))
	if err != nil {
		return err
	}
	pkgScope, err := qg.NewPackageScope(qs.Packages())
	if err != nil {
		return err
	}
	resolver := qg.NewProductionToolResolver()

	lt, err := qg.NewLintToolchain(
		qg.LintConfig(".golangci.yml"),
		qg.LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err != nil {
		return err
	}
	if err := qg.Lint(ctx, runner, resolver, fileOps, root, pkgScope, lt); err != nil {
		return err
	}
	if err := qg.Deadcode(
		ctx, runner, resolver, fileOps, root, pkgScope,
		qg.DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
		qg.DeadcodeArgs("-test", "-tags=yourtag"),
	); err != nil {
		return err
	}
	if err := qg.Vet(ctx, runner, fileOps, root, pkgScope); err != nil {
		return err
	}
	if err := qg.Compile(ctx, runner, fileOps, root, pkgScope); err != nil {
		return err
	}

	testOut, err := qg.Test(ctx, runner, store, fileOps, root, pkgScope)
	if err != nil {
		return err
	}
	if err := qg.Duration(ctx, runner, store, fileOps, root, testOut, qg.MaxSeconds(1.0)); err != nil {
		return err
	}

	inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, root, qs)
	if err != nil {
		return err
	}
	coveredOut, err := qg.CoveredTest(ctx, runner, store, fileOps, root, pkgScope, qs, inv)
	if err != nil {
		return err
	}
	covOut, err := qg.Coverage(ctx, runner, store, fileOps, root, coveredOut, qg.MinPercent(90))
	if err != nil {
		return err
	}
	if err := qg.Crap(ctx, runner, resolver, store, fileOps, root, covOut, inv, qg.MaxScore(8),
		qg.GocycloToolSpec("github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"),
	); err != nil {
		return err
	}
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}
	scanOut, err := mr.Scan(ctx, root, qs, inv,
		qg.GremlinsToolSpec("github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"),
	)
	if err != nil {
		return err
	}
	if err := qg.MutationSites(scanOut, qg.MaxSites(50)); err != nil {
		return err
	}
	return nil
}
```

## Typed Dependencies

The library uses output tokens to enforce dependency chains at compile time.

```go
// This won't compile — Crap requires CoverageOutput, not TestOutput
_ = qg.Crap(ctx, runner, resolver, store, fileOps, root, testOut, inv, qg.MaxScore(8),
	qg.GocycloToolSpec("github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"))
```

## Multiple Test Suites

Run separate test suites with different `QualityScope` values. Each `Test` returns its own `TestOutput`; use the matching token for each `Duration` check (duration validates all events in that run, not quality-scope excludes). For coverage and Crap, pick the suite whose `CoveredTestOutput` reflects the packages you want to measure (illustrative consumer layout: production code under `./cmd/...` — this module has no `cmd/` tree).

```go
//go:build mage

package main

import (
	"context"
	"os"

	qg "github.com/hotchkj/mage-gate/gate"
)

func GateMultiSuite() error {
	ctx := context.Background()
	runner, err := qg.NewDisplayRunner(qg.NewProductionRunner(), qg.OutputModeAgent, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	store := qg.NewArtifactStore()
	fileOps := qg.NewProductionFileOps()
	root := "."

	pkgScopeAll, err := qg.NewPackageScope("./...")
	if err != nil {
		return err
	}
	qs1, err := qg.NewQualityScope("./cmd/...", qg.Exclude("testdata"))
	if err != nil {
		return err
	}
	qs2, err := qg.NewQualityScope("./internal/...", qg.Exclude("fixtures"))
	if err != nil {
		return err
	}
	pkgScope1, err := qg.NewPackageScope(qs1.Packages())
	if err != nil {
		return err
	}
	pkgScope2, err := qg.NewPackageScope(qs2.Packages())
	if err != nil {
		return err
	}
	resolver := qg.NewProductionToolResolver()

	lt, err := qg.NewLintToolchain(
		qg.LintConfig(".golangci.yml"),
		qg.LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err != nil {
		return err
	}
	if err := qg.Lint(ctx, runner, resolver, fileOps, root, pkgScopeAll, lt); err != nil {
		return err
	}
	if err := qg.Compile(ctx, runner, fileOps, root, pkgScopeAll); err != nil {
		return err
	}

	// Suite 1: cmd package tests
	test1Out, err := qg.Test(ctx, runner, store, fileOps, root, pkgScope1)
	if err != nil {
		return err
	}
	if err := qg.Duration(ctx, runner, store, fileOps, root, test1Out, qg.MaxSeconds(2.0)); err != nil {
		return err
	}

	// Suite 2: internal package tests
	test2Out, err := qg.Test(ctx, runner, store, fileOps, root, pkgScope2)
	if err != nil {
		return err
	}
	if err := qg.Duration(ctx, runner, store, fileOps, root, test2Out, qg.MaxSeconds(1.0)); err != nil {
		return err
	}

	// Coverage and Crap from suite 1 token (production coverage for cmd)
	inv1, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, root, qs1)
	if err != nil {
		return err
	}
	coveredOut, err := qg.CoveredTest(ctx, runner, store, fileOps, root, pkgScope1, qs1, inv1)
	if err != nil {
		return err
	}
	covOut, err := qg.Coverage(ctx, runner, store, fileOps, root, coveredOut, qg.MinPercent(90))
	if err != nil {
		return err
	}
	if err := qg.Crap(ctx, runner, resolver, store, fileOps, root, covOut, inv1, qg.MaxScore(8),
		qg.GocycloToolSpec("github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"),
	); err != nil {
		return err
	}
	return nil
}
```

## Stage Functions

Package-level functions in `gate`. Each takes `context.Context`, then individual dependencies (`runner`, `fileOps`, `root`, and `store` where needed), then stage-specific inputs. `Lint` and `Format` take a `LintToolchain` assembled via `NewLintToolchain(config, tool, opts...)`.

| Function | Returns | Description |
|----------|---------|-------------|
| `Lint(ctx, runner, resolver, fileOps, root, packages, toolchain)` | `error` | Runs golangci-lint verify (`run`) |
| `Format(ctx, runner, resolver, fileOps, root, packages, toolchain)` | `error` | Applies configured formatters (`fmt`) |
| `Deadcode(ctx, runner, resolver, fileOps, root, packages, toolSpec, opts...)` | `error` | Detects unreachable code |
| `Vet(ctx, runner, fileOps, root, packages, opts...)` | `error` | Runs `go vet` |
| `Compile(ctx, runner, fileOps, root, packages, opts...)` | `error` | Verifies compilation with `go build` (no release binaries) |
| `Test(ctx, runner, store, fileOps, root, packages, opts...)` | `(TestOutput, error)` | Runs `go test` once without coverage flags |
| `QualityScopeInventory(ctx, runner, store, fileOps, root, qualityScope)` | `(QualityScopeInventoryOutput, error)` | Discovers and stores reusable quality-scope package inventory |
| `CoveredTest(ctx, runner, store, fileOps, root, packages, qualityScope, inventory, opts...)` | `(CoveredTestOutput, error)` | Runs coverage-bearing test pass for Coverage/Crap |
| `Coverage(ctx, runner, store, fileOps, root, coveredOutput, minPercent)` | `(CoverageOutput, error)` | Checks coverage threshold |
| `Crap(ctx, runner, resolver, store, fileOps, root, covOut, inventory, maxScore, gocycloToolSpec, opts...)` | `error` | Checks Crap score |
| `Duration(ctx, runner, store, fileOps, root, testOut, qg.MaxSeconds(secs))` | `error` | Checks each test completion event from the producing test run; quality-scope excludes do not apply; `qg.MaxSeconds` is required |
| `NewMutationRunner(runner, resolver, store, fileOps)` | `(MutationRunner, error)` | Shared gremlins scan/kill execution surface; nil deps rejected at construction |
| `MutationRunner.Scan(ctx, root, qualityScope, inventory, gremlinsToolSpec, opts...)` | `(MutationScanOutput, error)` | Gremlins dry-run on a `MutationRunner` value; token carries store binding for downstream checks |
| `MutationSites(scanOut, qg.MaxSites(n))` | `error` | Site budget over `MutationScanOutput` from `Scan` |
| `MutationCoverage(scanOut, qg.MinMutationCoverage(percent))` | `error` | Optional mutation coverage share over the same scan token |
| `MutationKills(ctx, runner, resolver, store, fileOps, root, qualityScope, inventory, qg.MinKillRate(percent), gremlinsToolSpec, opts...)` | `(MutationKillsOutput, error)` | On-demand mutation kill rate check (not part of Gate) |

Threshold arguments are option values (`qg.MinPercent`, `qg.MaxScore`, `qg.MaxSeconds`, `qg.MaxSites`, `qg.MinMutationCoverage`), not untyped literals passed positionally.

### Mutation analysis (dry-run producer, checks, and full run)

Mutation tooling is five separable concepts:

1. **Inventory producer** — `QualityScopeInventory` runs package discovery once and stores `quality-scope-package-rows.json` with a scope fingerprint; coverage, CRAP, mutation scan, and mutation kill consumers receive the typed inventory token and validate fingerprint match.
2. **Dry-run producer** — `MutationRunner.Scan` runs gremlins with `--dry-run` once and writes the mutation artifact. **QualityScope** and its inventory define that run (package seed, build tags, excludes, test-file patterns); it is the measurement boundary for mutation metrics, not the same thing as `PackageScope` run targets for lint or `go test`.
3. **Site budget** — `MutationSites(scanOut, qg.MaxSites(n))` enforces the per-file mutation-site cap from that scan’s artifact.
4. **Mutation coverage** — `MutationCoverage(scanOut, qg.MinMutationCoverage(p))` enforces the covered-mutant share from the **same** dry-run artifact. Optional; when enabled, it still does not require a second gremlins dry-run.
5. **Kill rate** — A full gremlins run (`MutationRunner.Kill` + `MutationKillRate` on the kill token, or the bundled `MutationKills` entrypoint) measures whether tests kill mutants. That path is **not** a substitute for the dry-run metrics unless callers explicitly use `MutationSitesFromKills` / `MutationCoverageFromKills` over full-run output.

**Composition:** One `Scan` can supply both the site-budget check and, when you configure it, the mutation-coverage check—one dry-run, two quality checks. **MutationKills** (full run) remains separate: optional, on-demand, and typically a dedicated magefile target (e.g. merge or deeper quality), not the same cost profile as pre-commit dry-run checks.

## Options

```go
// Coverage minimum threshold (required)
qg.MinPercent(95)

// Maximum CRAP score (required)
qg.MaxScore(5)

// Maximum test duration (required)
qg.MaxSeconds(2.0)

// Maximum mutation sites per file (required)
qg.MaxSites(100)

// QualityScope excludes and test file patterns (options mutate private qualityScopeConfig;
// package pattern is required identity set only by NewQualityScope)
// qg.NewQualityScope("./...", qg.Exclude("internal/test", "tools"), qg.TestFilePatterns("*_test.go"))

// CompileArgs for go build flags on Compile (e.g. -trimpath); artifact builds are consumer-owned.
qg.CompileArgs("-trimpath")

// Vet and test passthrough args
qg.VetArgs("-json")
qg.TestArgs("-tags=integration", "-short")

// Lint config path (required)
qg.LintConfig(".golangci.yml")

// Custom golangci-lint definition and explicit builder pin
qg.CustomGCL(".custom-gcl.yml")
qg.CustomLintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1")

// Extra golangci-lint argv (after subcommand prefix: run … for Lint, fmt … for Format)
qg.LintArgs("--verbose")

// Deadcode: optional args for the deadcode tool. Include "-test" for test-inclusive
// reachability (required for library repos with no main packages).
qg.DeadcodeArgs("-test", "-tags=integration")

// Extra gocyclo argv for Crap
qg.CrapArgs("-top", "10")

// Gremlins unleash extras (MutationSites / MutationKills); sites still inject --dry-run first.
qg.MutationArgs("--timeout=10m")
```

## Pre-commit Gate

Skip coverage and Crap; run a smaller set of stages (lint, compile, test, mutation site budget via gremlins `--dry-run`):

```go
//go:build mage

package main

import (
	"context"
	"os"

	qg "github.com/hotchkj/mage-gate/gate"
)

func PreCommit() error {
	ctx := context.Background()
	runner, err := qg.NewDisplayRunner(qg.NewProductionRunner(), qg.OutputModeAgent, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	store := qg.NewArtifactStore()
	fileOps := qg.NewProductionFileOps()
	root := "."

	qs, err := qg.NewQualityScope("./...", qg.Exclude("features", "testdata"))
	if err != nil {
		return err
	}
	pkgScope, err := qg.NewPackageScope(qs.Packages())
	if err != nil {
		return err
	}
	resolver := qg.NewProductionToolResolver()

	lt, err := qg.NewLintToolchain(
		qg.LintConfig(".golangci.yml"),
		qg.LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err != nil {
		return err
	}
	if err := qg.Lint(ctx, runner, resolver, fileOps, root, pkgScope, lt); err != nil {
		return err
	}
	if err := qg.Compile(ctx, runner, fileOps, root, pkgScope); err != nil {
		return err
	}
	if _, err := qg.Test(ctx, runner, store, fileOps, root, pkgScope); err != nil {
		return err
	}
	inv, err := qg.QualityScopeInventory(ctx, runner, store, fileOps, root, qs)
	if err != nil {
		return err
	}
	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}
	scanOut, err := mr.Scan(ctx, root, qs, inv,
		qg.GremlinsToolSpec("github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"),
	)
	if err != nil {
		return err
	}
	if err := qg.MutationSites(scanOut, qg.MaxSites(50)); err != nil {
		return err
	}
	return nil
}
```

## Output Modes

Output mode is a property of the runner, not the stage. Construct the runner with the desired mode via `NewDisplayRunner`.

### Silent display mode (default)

Silent on success; failures show ERROR/Fix/Hint format:

```go
runner, err := qg.NewDisplayRunner(qg.NewProductionRunner(), qg.OutputModeAgent, os.Stdout, os.Stderr)
if err != nil {
	return err
}
```

```text
ERROR: [coverage] stage coverage failed
Fix: check coverage stage configuration
Hint: see coverage stage artifacts for details
```

### Verbose display mode

Full subprocess output on the display writers (e.g. CI logs):

```go
runner, err := qg.NewDisplayRunner(qg.NewProductionRunner(), qg.OutputModeVerbose, os.Stdout, os.Stderr)
if err != nil {
	return err
}
```

## Diagnostic Troubleshooting

### Coverage Gate Fails

If coverage is below the threshold:

```text
ERROR: [coverage] stage coverage failed
Fix: check coverage stage configuration
Hint: see coverage stage artifacts for details
```

**Action**: Write tests for uncovered code paths. Use `go tool cover -html=coverage.out` to visualize coverage.

### Crap Analysis Fails

If any function exceeds the Crap threshold:

```text
ERROR: [crap] stage crap failed
Fix: check crap stage configuration
Hint: see crap stage artifacts for details
```

**Action**: Either reduce cyclomatic complexity or add tests to improve coverage for complex functions.

### Unit Tests Exceed Duration Budget

If any test runs longer than the configured maximum:

```text
ERROR: [duration] stage duration failed
Fix: check duration stage configuration
Hint: see duration stage artifacts for details
```

**Action**: Profile slow tests and optimize.

### Mutation Sites Exceed Budget

If the code has too many viable mutation sites:

```text
ERROR: [mutationsites] mutationsites failed
Fix: reduce mutation site count by splitting large files
Hint: rule of thumb: split by theme, move at least 30% of code out
```

**Action**: Split large files by theme to reduce mutation site density.

## Use in another repository

1. Add a dependency on the **module root** (not only the `gate` subfolder): `go get github.com/hotchkj/mage-gate@vX.Y.Z` (or a pseudo-version from `main` until the first tag exists).
2. Add a `magefiles/` package with `//go:build mage` and a `Gate()` function that wires `gate` the way you want (this repo’s `magefiles/magefile.go` + `gate.toml` are the reference implementation).
3. Run `go run github.com/magefile/mage@v1.17.0 gate` from the repo root (Mage discovers `magefiles/` automatically).

For local development against a clone of this repo, use a temporary `replace` in the consumer’s `go.mod`:

```txt
replace github.com/hotchkj/mage-gate => ../mage-gate
```

Adjust the relative path if your clone is not a sibling directory. Remove `replace` before tagging the consumer for release.

## Contributing

### Building

This module is a library (no `go get`-able CLI in `cmd/`). To compile packages locally:

```bash
go build ./gate/... ./cmdrunner/... ./internal/...
```

Release binaries, cross-compilation, and packaging are consumer-owned — the gate verifies quality only.

### Quality Gate

Run the full quality gate before submitting changes:

```bash
go run github.com/magefile/mage@v1.17.0 gate
```

Apply formatters separately (same `[lint]` toolchain as `mage lint`):

```bash
go run github.com/magefile/mage@v1.17.0 format
```

The repo pre-commit hook (`.githooks/pre-commit`) runs **`mage format` then `mage gate`**: apply first, then verify. `gate` itself stays non-mutating (no Format step in `runGateSteps`).
