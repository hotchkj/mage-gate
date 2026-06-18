---
name: ""
overview: ""
todos: []
isProject: false
---

---
name: issue-21-complete-fix
overview: "Replace the incomplete boundary-audit plan with an issue-level fix for #21 that is lint-clean, explicitly records decisions, and verifies against the full gate before commit."
todos:
  - id: api-contract
    content: Lock the verbatim public API contract and forbidden new APIs before implementation
    status: pending
  - id: bdd-contract
    content: Apply the exact BDD feature contract for duration and scope-bound inventory behavior
    status: pending
  - id: issue-closure
    content: Complete or explicitly document every issue 21 item with tests/docs tied to each decision
    status: pending
  - id: qualityscope-handle
    content: Refactor QualityScope into a small immutable constructor-built handle with no hugeParam suppressions
    status: pending
  - id: drift-controls
    content: Run task-executor plus critical-reviewer gates using the issue matrix, API diff, BDD diff, lint, tests, and full gate evidence
    status: pending
  - id: commit-slices
    content: Split implementation into reviewable commits with per-commit verification and no 50-file omnibus commit
    status: pending
  - id: salvage-branch
    content: Preserve the current dirty 56-file work in a Git salvage branch, then rebuild clean reviewable commits from it
    status: pending
  - id: concise-execution
    content: Keep execution concise: no broad rereads, no subagents, no long status narration unless explicitly requested
    status: pending
isProject: false
---

# Issue 21 Complete Boundary Fix

## Non-Negotiable Outcome

This plan closes GitHub issue #21 as written, not the old seven-item implementation slice. The final diff must contain an explicit issue #21 closure note in `docs/mage-gate-intent-and-design.md` or the plan file showing all twelve issue areas as fixed, documented runtime compromise, or documented consumer policy.

No new public gate steps are introduced. No compatibility shims are introduced. No `//nolint:gocritic` is added for `QualityScope` size. No linter config threshold is raised.

## Concise Execution Requirement

Execution must be as concise as possible:

- No broad repo or issue rereads after this plan is accepted unless a specific verification failure creates a concrete unknown.
- No subagents unless explicitly approved by the user for a named slice.
- No long explanatory status updates during execution. Report only the current slice, files changed, verification command/result, commit hash, or blocker.
- No repeated plan rewrites unless the public API contract, BDD contract, commit slicing, or recovery workflow changes.
- Prefer Git diff/status/test output over prose analysis. If a command proves a fact, cite the command and result instead of re-explaining the codebase.
- Stop after each commit slice unless the user explicitly approves continuing through multiple slices in one turn.

## Recovery From Current Dirty Tree

The current 56-file dirty tree must not become a review commit. Preserve it in Git, then rebuild.

### Salvage Branch

1. Create a salvage branch from the current dirty state:

```powershell
git switch -c salvage/issue-21-dirty-boundary-audit
```

2. Stage the current tracked changes and the replacement plan only. Do not include unrelated local files unless explicitly approved:

```powershell
git add README.md docs/ features/ gate/ gatetest/ integration/ internal/ magefiles/
git add c:\Users\Jeff\.cursor\plans\issue-21-complete-fix_76fd8a79.plan.md
```

3. Create a salvage commit with hooks enabled if possible. If hooks fail because the dirty tree is known not to pass, stop and ask for approval before using a no-hook salvage commit. The salvage commit message must make clear it is not reviewable product work:

```text
salvage: preserve dirty issue 21 boundary audit attempt

Not for review as final implementation. This commit preserves the current
working tree so the real issue #21 fix can be rebuilt into reviewable slices.
```

The salvage branch is the backup and audit trail. Do not use patch files as the primary recovery mechanism.

### Clean Implementation Branch

After the salvage commit exists, return to the original base branch and create the real implementation branch:

```powershell
git switch -
git switch -c fix/issue-21-boundary-contract
```

If returning to the original branch requires discarding dirty working-tree contents, ask for explicit approval before any destructive command. The approval request must name the salvage branch and commit that preserves the work.

### Rebuild From Salvage

Use Git to pull only the relevant hunks/files from `salvage/issue-21-dirty-boundary-audit` into each planned commit:

```powershell
git restore --source salvage/issue-21-dirty-boundary-audit -- path/to/file
git restore -p --source salvage/issue-21-dirty-boundary-audit -- path/to/file
git checkout -p salvage/issue-21-dirty-boundary-audit -- path/to/file
```

Do not apply the salvage commit wholesale. Each real commit is reconstructed from clean base according to the commit slicing section below.

## Verbatim Public API Contract

These are the only public API signature changes allowed:

```go
func Duration(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	testOutput TestOutput,
	maxSeconds DurationThreshold,
) (err error)
```

```go
func NewMutationRunner(
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
) (MutationRunner, error)
```

These public signatures must remain unchanged:

```go
func NewQualityScope(pkgs string, opts ...QualityScopeOption) (QualityScope, error)
func (s QualityScope) Packages() string
func (s QualityScope) Tags() []string
func (s QualityScope) ExcludeSegments() []string
func (s QualityScope) TestFilePatterns() []string
type QualityScopeOption func(*qualityScopeConfig)
func Tags(tags ...string) QualityScopeOption
func Exclude(segments ...string) QualityScopeOption
func TestFilePatterns(patterns ...string) QualityScopeOption
```

```go
func NewPackageScope(pattern string) (PackageScope, error)
func (p PackageScope) Packages() string
```

```go
func Lint(ctx context.Context, runner CommandRunner, resolver ToolResolver, fileOps FileOps, root string, packages PackageScope, toolchain LintToolchain) (err error)
func Format(ctx context.Context, runner CommandRunner, resolver ToolResolver, fileOps FileOps, root string, packages PackageScope, toolchain LintToolchain) (err error)
func Compile(ctx context.Context, runner CommandRunner, fileOps FileOps, root string, packages PackageScope, opts ...CompileOption) (err error)
func Vet(ctx context.Context, runner CommandRunner, fileOps FileOps, root string, packages PackageScope, opts ...VetOption) (err error)
func Deadcode(ctx context.Context, runner CommandRunner, resolver ToolResolver, fileOps FileOps, root string, packages PackageScope, deadcodeSpec DeadcodeToolValue, opts ...DeadcodeOption) (err error)
func MarkdownLint(ctx context.Context, runner CommandRunner, resolver ToolResolver, fileOps FileOps, root string, toolSpec MarkdownLintToolValue, opts ...MarkdownLintOption) (err error)
func Test(ctx context.Context, runner CommandRunner, store *ArtifactStore, fileOps FileOps, root string, packages PackageScope, opts ...TestOption) (out TestOutput, err error)
func CoveredTest(ctx context.Context, runner CommandRunner, store *ArtifactStore, fileOps FileOps, root string, packages PackageScope, production QualityScope, inventory QualityScopeInventoryOutput, opts ...TestOption) (out CoveredTestOutput, err error)
func QualityScopeInventory(ctx context.Context, runner CommandRunner, store *ArtifactStore, fileOps FileOps, root string, qualityScope QualityScope) (out QualityScopeInventoryOutput, err error)
```

No `DurationFromCovered`, builder, alternate runner constructor, or new package-scope variant is allowed in this fix.

## Verbatim BDD Contract

Delete the old duration behavior scenario from `features/duration.feature`:

```gherkin
Scenario: quality scope excludes narrow the duration boundary
```

Add this replacement scenario to `features/duration.feature`:

```gherkin
Scenario: duration checks all test events from the producing test run
  Given the codebase has 95% test coverage
  And the quality scope excludes "testutil"
  And the output mode is agent
  And the duration threshold is 1.0 seconds
  And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
  And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
  And package "example.com/mod/zz_bdd_not_default/testutil" has a test event "TestSlowUtility" lasting 2.0 seconds
  When the gate runs steps qualityscopeinventory, coveredtest, duration
  Then the step fails
  And the error is ErrDurationFailed
```

If the existing step vocabulary lacks `has a test event`, implement that BDD step in `features/internal/steps/*` with in-memory artifacts only. Do not add production API for this.

Do not invent new Gherkin step names unless the exact wording above cannot be supported by the existing BDD state model; if wording must change, update this plan first before implementation.

## Issue 21 Closure Matrix

- Problem 1, `QualityScopeOption` mutable identity: fix by keeping `QualityScopeOption func(*qualityScopeConfig)` where `qualityScopeConfig` excludes package identity. Add/keep a unit test proving in-package options cannot access `packages` by type shape is not possible directly, so evidence is constructor code plus no option receiving `*QualityScope`.
- Problem 2, CSV storage: fix by storing tags, excludes, and test-file patterns as slices inside immutable private data. Render CSV only at command argv seams.
- Problem 3, zero-constructible `PackageScope` and `QualityScope`: accept as documented Go limitation. `QualityScope{}` must be invalid because its private data pointer is nil. `PackageScope{}` remains invalid via runtime validation. Docs must list this as accepted runtime-only validation.
- Problem 4, package-pattern boundary: trim at construction and store the canonical trimmed package pattern in both `PackageScope` and `QualityScope`. Whitespace-only and leading flags reject at construction.
- Problem 5, threshold/tool-spec zero values: accept runtime `set`/`valid` flags as Go zero-value limitation. Add missing doc comments to threshold and tool-spec value types, including `GocycloToolValue`, `GremlinsToolValue`, `DeadcodeToolValue`, `MarkdownLintToolValue`, `LintConfigValue`, and `LintToolValue`.
- Problem 6, inventory identity: implement strict scope fingerprinting. Consumers reject mismatched scope/inventory with `ErrQualityScopeInventoryMismatch`.
- Problem 7, duration scope drift: remove the `QualityScope` parameter from `Duration`. Duration validates only `TestOutput` and threshold, then checks all test events in that run.
- Problem 8, gate-to-harness primitive collapse: keep primitive thresholds/tool specs at the harness external-process boundary; use `QualityScopeCommandScope` for quality-scope-aware coverage, CRAP, and mutation command projection. Document this as the deliberate internal seam.
- Problem 9, `NewMutationRunner` nil deps: constructor returns `(MutationRunner, error)` and rejects nil runner, resolver, store, and fileOps immediately.
- Problem 10, silent defaults: public `gate` rejects empty/whitespace root. `internal/harness` must not silently default an empty package string for package-running steps; callers must pass package scope explicitly. Magefile `./...` default is retained only as this repository consumer policy and must be documented/test-covered in `magefiles/config.go`.
- Problem 11, public option pattern: keep exported `*Option` types only for optional argv/config assembly. Typed options remain only when they encode a gate invariant, currently `QualityScopeOption` for tags/excludes/test-file patterns and `LintOption` for assembling `LintToolchain`. Document no major-version API break for this item.
- Problem 12, BDD/support runtime patterns: ordinary BDD setup builds typed prerequisites once and fails early when missing. Zero-value tokens remain only in negative tests whose scenario is explicitly validation failure.

## QualityScope Design

`QualityScope` must be a small public value token backed by private immutable data:

```go
type QualityScope struct {
	data *qualityScopeData
}
```

The private data owns canonical package identity and the slice fields. `NewQualityScope` validates, trims, copies all optional slices, and stores the data. Getters return defensive copies. Internal code must use private accessors rather than scattered direct struct-field reads.

Zero value behavior:

```go
var qs QualityScope
```

is invalid at step entry because `qs.data == nil`; it must not panic.

## Implementer Drift Controls

Before coding, the executor must paste this checklist into their working notes and check each item off with evidence:

- Public API diff matches only the two allowed signature changes above.
- No new public functions, steps, builders, or compatibility shims were introduced.
- `features/duration.feature` contains the exact replacement duration scenario or this plan was updated before implementation.
- Every issue #21 item has a matching test/doc/code evidence line.
- `QualityScope` is lint-clean because it is small, not because lint was suppressed.
- The old `.cursor/plans/gate_options_boundary_audit_d5f40e27.plan.md` is either updated to point at this replacement plan or excluded from the commit.

## Commit Slicing

Do not produce one omnibus commit. The implementation must be split into these reviewable commits, in this order. Each commit must compile and pass its listed verification before moving to the next.

### Commit 1: Plan And Issue Contract

Purpose: make the execution/review contract visible before code changes.

Files:
- `c:\Users\Jeff\.cursor\plans\issue-21-complete-fix_76fd8a79.plan.md`
- optionally `d:\Git\mage-gate\.cursor\plans\gate_options_boundary_audit_d5f40e27.plan.md` only to mark it superseded and point to the replacement plan

Verification:
- `git diff --check`

### Commit 2: QualityScope Handle And Pattern Canonicalization

Purpose: fix the structural `hugeParam` failure and issue #21 problems 1, 2, 3, and 4 before touching step behavior.

Files should be limited to:
- `gate/quality_scope.go`
- `gate/package_scope.go`
- `gate/pattern_validate.go`
- focused tests in `gate/*quality_scope*_test.go`, `gate/pattern_validate_test.go`, and token tests that need private accessor updates

Required evidence:
- `QualityScope` is a small by-value handle backed by private immutable data.
- `NewQualityScope("  ./...  ")` and `NewPackageScope("  ./...  ")` store `"./..."`.
- Zero-value `QualityScope{}` still returns validation errors, not panics.
- No new `QualityScope` `hugeParam` suppressions.

Verification:
- `go run github.com/magefile/mage@v1.17.0 lint`
- `go test ./gate/... -count=1`

### Commit 3: Inventory Identity And Command-Scope Boundary

Purpose: fix issue #21 problems 6 and 8 for quality-scope-aware steps.

Files should be limited to:
- `gate/step.go`
- `gate/steps_quality_scope_inventory.go`
- `gate/steps_coverage.go`
- `gate/steps_crap_duration.go` only for CRAP-related scope access, not Duration behavior
- `gate/mutation_kills_public.go`
- `internal/gatecheck/quality_scope_command_scope.go`
- `internal/harness/*coverage*`, `internal/harness/*crap*`, `internal/harness/*mutation*` only where needed for command-scope projection
- focused inventory/command-scope tests

Required evidence:
- Inventory/scope mismatches reject with `ErrQualityScopeInventoryMismatch`.
- Tags/excludes/test-file patterns render to primitives only at command argv or parsing seams.

Verification:
- `go test ./gate/... ./internal/gatecheck/... ./internal/harness/... -count=1`
- `go run github.com/magefile/mage@v1.17.0 lint`

### Commit 4: Duration Public API And BDD Contract

Purpose: fix issue #21 problem 7 with the exact API and BDD behavior in this plan.

Files should be limited to:
- `features/duration.feature`
- `features/internal/steps/*` needed for the exact BDD scenario
- `gate/steps_crap_duration.go` for `Duration`
- `internal/harness/step_duration.go`
- duration tests and call sites in `README.md`, `magefiles/`, `integration/`, and `gate/*duration*`

Required evidence:
- `Duration` has exactly the planned signature.
- Old exclude-narrows-duration scenario is gone.
- Replacement scenario proves duration checks all test events from `TestOutput`.
- No new production step or public API is added.

Verification:
- `go test ./features/... -count=1`
- `go test ./gate/... ./internal/harness/... -count=1`
- `go run github.com/magefile/mage@v1.17.0 lint`

### Commit 5: MutationRunner Fail-Fast

Purpose: fix issue #21 problem 9.

Files should be limited to:
- `gate/mutation_runner.go`
- mutation runner tests
- direct call sites in `features/`, `magefiles/`, `integration/`, `README.md`

Required evidence:
- `NewMutationRunner` has exactly the planned `(MutationRunner, error)` signature.
- Nil dependencies fail at construction with `ErrNilDependency`.
- `Scan` and `Kill` do not duplicate constructor dependency validation.

Verification:
- `go test ./gate/... ./features/... ./magefiles/... ./integration/... -count=1`
- `go run github.com/magefile/mage@v1.17.0 lint`

### Commit 6: Defaults, Option Policy, Documentation

Purpose: fix/document issue #21 problems 5, 10, 11, and 12 without mixing behavior into earlier commits.

Files should be limited to:
- `docs/mage-gate-intent-and-design.md`
- `gate/options.go`
- `gate/lint_toolchain.go`
- `gate/steps.go`
- `gate/steps_public.go`
- `internal/harness/step_runner.go` and direct package-default tests if removing harness defaults
- `magefiles/config.go` and tests if documenting retained consumer policy
- BDD/support cleanup files only where ordinary setup still hides missing prerequisites

Required evidence:
- Tool-spec and threshold zero-value compromises are documented.
- Public `*Option` policy is documented.
- Empty public root rejects.
- Empty harness package string no longer silently defaults for package-running steps, or the only remaining default is documented consumer policy outside `gate`/`internal/harness`.
- BDD zero-value tokens are limited to negative validation paths.

Verification:
- `go test ./gate/... ./features/... ./internal/harness/... ./magefiles/... -count=1`
- `go run github.com/magefile/mage@v1.17.0 lint`

### Commit 7: Final Gate Proof

Purpose: only final verification and any formatting changes produced by the hook.

Files:
- No intentional source changes except formatter output from the prior commit, if any.

Verification:
- `go run github.com/magefile/mage@v1.17.0 format`
- `go run github.com/magefile/mage@v1.17.0 gate`
- `git status --short`

If any commit touches substantially more files than its listed scope, stop and split it further before committing.

## Subagent Verification

Use a task-executor subagent only after this plan is accepted. Give it this plan verbatim and require it to return:

- The exact public API diff it produced.
- The exact BDD diff it produced.
- The issue #21 closure evidence list, one line per problem.
- The commit slice it is implementing and whether changed files stayed within that slice.
- The commands it ran and their exit codes.

Then run a separate critical-reviewer subagent with this fixed prompt:

```text
Review the current working tree against c:\\Users\\Jeff\\.cursor\\plans\\issue-21-complete-fix_76fd8a79.plan.md and GitHub issue #21. Block completion if: public API diff differs from the plan, BDD feature diff differs from the plan, any issue #21 item lacks code/test/doc evidence, any QualityScope hugeParam suppression was added, or full mage gate evidence is missing.
```

Parent agent must independently verify the same command set after subagents finish. Subagent output is evidence, not delegation of responsibility.

## Verification Commands

Run in this order, stopping on first failure:

```powershell
go run github.com/magefile/mage@v1.17.0 format
go run github.com/magefile/mage@v1.17.0 lint
go test ./gate/... -count=1
go test ./features/... -count=1
go test ./internal/gatecheck/... ./internal/harness/... -count=1
go test ./magefiles/... ./integration/... -count=1
go run github.com/magefile/mage@v1.17.0 gate
git status --short
```

Only after all commands pass may the commit be attempted with hooks enabled. If the hook changes files, re-run the verification commands before committing again.