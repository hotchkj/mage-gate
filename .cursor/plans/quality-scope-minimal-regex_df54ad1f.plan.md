---
name: quality-scope-minimal-regex
overview: Extend the existing quality-scope command scope with a focused completion phase for minimal mutation exclude regexes and cleanup of local trace artifacts. The old plan remains authoritative; this phase reconciles implementation and tests with its minimal-regex contract.
todos:
  - id: validate-existing-plan
    content: Treat `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` as authoritative and add Phase 5 for minimal mutation regex completion.
    status: completed
  - id: bdd-minimal-regex
    content: Update mutation BDD expectations from concrete test-file flags to minimal per-package test-pattern regexes.
    status: completed
  - id: gatecheck-minimizer
    content: Redesign `internal/gatecheck/gremlins_exclude.go` to emit minimal safe regexes for test-file patterns while preserving concrete file/path excludes where required.
    status: completed
  - id: parity-tests
    content: Update plan, harness, and public inventory consumer parity tests to assert shared filters and minimal argv across commands.
    status: completed
  - id: gotmp-cleanup
    content: Ensure verification/tracing does not create `.gotmp` artifacts or require ignore-file changes.
    status: completed
  - id: verify-full-gate
    content: Run targeted BDD/unit tests, lint, and full `mage gate` with normal thresholds restored.
    status: completed
isProject: false
---

# Complete Quality Scope Minimal Regex Plan

## Existing Plan Status

The existing plan at `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` still applies. Its non-negotiable contract is the source of truth:

- `QualityScope` is the canonical measurement statement.
- One `QualityScopeInventory` plus one `QualityScope` produces one internal command scope.
- Minimal command inputs are required: package CSVs for package tools, directory regexes when a directory excludes all children, and file regexes only when a wider exclusion would remove in-scope code.
- Mutation commands must use `--exclude-files=<minimal-regex>`.
- Coverage, CRAP, mutation scan/kill, `MutationSites`, and `MutationCoverage` must consume the same parsed filters.

The intent doc at `docs/mage-gate-intent-and-design.md` agrees with this boundary. It says gremlins exclude arguments are regex strings built from canonical root-relative inventories, and test file patterns identify files like `*_test.go` to skip inside otherwise in-scope production packages. It does not require per-file argv expansion.

## Governance And No-Drift Rules

This is a completion plan for existing committed code on the current branch. Implementers must repair the current implementation and tests; they must not redesign the public API, defer behavior to a future task, or reinterpret the old plan as optional.

Hard constraints:

- The existing plan `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` remains authoritative unless this completion plan explicitly tightens it.
- `docs/mage-gate-intent-and-design.md` is the semantic source for scope, path realms, and mutation strategy.
- `docs/kb/coding-standards.md`, `docs/kb/verification.md`, `docs/kb/issue-ownership.md`, `docs/kb/agent-delegation.md`, `docs/kb/code-smells.md`, and `docs/kb/error-handling.md` must be read before implementation or review work.
- BDD changes come before implementation. The first implementation edit must be preceded by failing BDD or unit evidence that demonstrates current per-file test regex enumeration violates this plan.
- The fix must modify existing committed production paths; it must not add a parallel helper, compatibility shim, fallback path, or feature flag.
- Expected argv must be literal in BDD. BDD expected-command helpers may normalize placeholders, but must not import, call, mirror, or derive expectations from `internal/gatecheck`.
- All command contracts in this plan are acceptance criteria. If a tool cannot express one exactly, the implementer must stop and update the plan with evidence rather than silently approximate it.
- Plan edits during execution may only clarify or tighten acceptance behavior. They must not weaken exact argv requirements, permit a second valid command shape, or defer a listed acceptance criterion.
- Conflict resolution: this completion plan supersedes the old plan only where it explicitly tightens behavior. Everywhere else, `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` remains binding.
- Any deviation from this plan requires a markdown plan edit before code changes.

## Drift To Correct

Current behavior violates the existing plan by expanding `test_file_patterns = ["*_test.go"]` into one `--exclude-files` flag per known test file. That is not minimal and does not satisfy “file regexes only where a wider exclusion would remove in-scope code.”

The existing BDD scenario named `test file patterns become concrete exclude-file arguments` is too narrow and now misleading. It proves one fixture file is excluded, but it accidentally blesses per-file enumeration. This should be reframed as minimal regex behavior while keeping actor-visible command assertions independent of production helpers.

## Phase 5: Minimal Regex Completion

Add a new phase to `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` after the cleanup/parity phase:

- Define the missing projection rule: test-file patterns in mutation argv collapse to the smallest safe package-root regex for each otherwise in-scope package directory.
- Preserve concrete file regexes for source-path excludes that are genuinely file-specific, such as root module files or source files under excluded non-package paths.
- Preserve directory regexes for package/path segment excludes.
- Keep all threshold filters (`CoverageProfileFilter`, CRAP parsing, mutation metrics) on the same parsed `ExcludeSegments` + `TestFilePatterns`; only the gremlins argv serialization changes.
- Keep `Duration` out of quality-scope command-scope parity, per the existing plan.

## Exact Dogfood Command Contract

The repo dogfood gate, using `gate.toml` as committed, must dispatch these quality-scope-relevant command argv shapes. Verification must compare the logged argv exactly after normalizing artifact paths and local-vs-`go run` tool resolution where the BDD harness intentionally abstracts that resolver choice.

- `QualityScopeInventory`
  - Command: `go`
  - Args:
    ```text
    list
    -e
    -tags=mage
    -f
    <package-inventory-format>
    ./...
    ```
  - Excludes and test-file patterns must not appear in this command.

- `CoveredTest`
  - Command: `go`
  - Args:
    ```text
    test
    ./...
    -json
    -coverprofile=<artifact>/coverage.out
    -coverpkg=github.com/hotchkj/mage-gate/cmdrunner,github.com/hotchkj/mage-gate/gate,github.com/hotchkj/mage-gate/internal/fileopspath,github.com/hotchkj/mage-gate/internal/fsnorm,github.com/hotchkj/mage-gate/internal/gatecheck,github.com/hotchkj/mage-gate/internal/harness,github.com/hotchkj/mage-gate/magefiles
    -tags=mage
    -shuffle=on
    -count=1
    ```
  - `-coverpkg` must be the exact minimal import CSV after quality-scope package/path excludes.
  - Test-file patterns must not change `-coverpkg`; they only activate profile and per-file analysis filters.

- `MutationRunner.Scan`
  - Command: local `gremlins` or `go run <gremlinsSpec>` depending on resolver state.
  - Args after the executable/pinned spec:
    ```text
    unleash
    -o
    <artifact>/mutations.json
    --coverpkg=github.com/hotchkj/mage-gate/cmdrunner,github.com/hotchkj/mage-gate/gate,github.com/hotchkj/mage-gate/internal/fileopspath,github.com/hotchkj/mage-gate/internal/fsnorm,github.com/hotchkj/mage-gate/internal/gatecheck,github.com/hotchkj/mage-gate/internal/harness,github.com/hotchkj/mage-gate/magefiles
    --tags=mage
    --exclude-files=^cmdtest(/|$)
    --exclude-files=^features(/|$)
    --exclude-files=^gatetest(/|$)
    --exclude-files=^integration(/|$)
    --exclude-files=^testdata(/|$)
    --exclude-files=^cmdrunner/.*_test\.go$
    --exclude-files=^gate/.*_test\.go$
    --exclude-files=^internal/fileopspath/.*_test\.go$
    --exclude-files=^internal/fsnorm/.*_test\.go$
    --exclude-files=^internal/gatecheck/.*_test\.go$
    --exclude-files=^internal/harness/.*_test\.go$
    --exclude-files=^magefiles/.*_test\.go$
    --dry-run
    ```
  - Exclude regex args MUST be sorted lexicographically by canonical regex text after required command arguments.
  - No `--exclude-files=^<package>/<file>_test\.go$` entries are allowed when a package-level test-pattern regex can express the same exclusion without excluding production files.
  - Concrete file regexes remain allowed only for file-specific path exclusions such as `^main\.go$`, metacharacter-bearing explicit files, or source files outside package inventory where no package-level regex is correct.
  - The `testdata` exclusion MUST be `--exclude-files=^testdata(/|$)`. It must not be `^testdata/.*` and must not enumerate files under `testdata`.

- `MutationRunner.Kill` and `MutationKills`
  - Command: local `gremlins` or `go run <gremlinsSpec>` depending on resolver state.
  - Args after the executable/pinned spec:
    ```text
    unleash
    -o
    <artifact>/mutations.json
    --coverpkg=github.com/hotchkj/mage-gate/cmdrunner,github.com/hotchkj/mage-gate/gate,github.com/hotchkj/mage-gate/internal/fileopspath,github.com/hotchkj/mage-gate/internal/fsnorm,github.com/hotchkj/mage-gate/internal/gatecheck,github.com/hotchkj/mage-gate/internal/harness,github.com/hotchkj/mage-gate/magefiles
    --tags=mage
    --exclude-files=^cmdtest(/|$)
    --exclude-files=^features(/|$)
    --exclude-files=^gatetest(/|$)
    --exclude-files=^integration(/|$)
    --exclude-files=^testdata(/|$)
    --exclude-files=^cmdrunner/.*_test\.go$
    --exclude-files=^gate/.*_test\.go$
    --exclude-files=^internal/fileopspath/.*_test\.go$
    --exclude-files=^internal/fsnorm/.*_test\.go$
    --exclude-files=^internal/gatecheck/.*_test\.go$
    --exclude-files=^internal/harness/.*_test\.go$
    --exclude-files=^magefiles/.*_test\.go$
    ```
  - This argv MUST be identical to `MutationRunner.Scan` except `--dry-run` MUST be absent.
  - Exclude regex ordering, concrete-file exceptions, and anti-enumeration rules are identical to scan.

- `Coverage`
  - Command: `go`
  - Args:
    ```text
    tool
    cover
    -func=<artifact>/coverage-filtered.out
    ```
  - Because `gate.toml` has both exclude segments and `test_file_patterns`, `coverage-filtered.out` is required. `coverage.out` is only valid when neither filter kind is active.

- `CRAP`
  - Complexity command: local `gocyclo` or `go run <gocycloSpec>` depending on resolver state.
  - Args after the executable/pinned spec:
    ```text
    -over
    0
    cmdrunner
    gate
    internal/fileopspath
    internal/fsnorm
    internal/gatecheck
    internal/harness
    magefiles
    ```
  - Cover function command:
    ```text
    go tool cover -func=<artifact>/coverage-filtered.out
    ```
  - Module metadata command:
    ```text
    go list -m -json
    ```
  - CRAP must parse gocyclo output using the same `ExcludeSegments` and `TestFilePatterns` as coverage and mutation metrics.

- `Duration`
  - No subprocess command is allowed for quality-scope command parity.
  - It reads the CoveredTest event artifact and applies duration semantics in-process.

- `Integration Test`
  - Command: `go`
  - Args:
    ```text
    test
    ./integration/...
    -json
    -tags=integration
    -count=1
    ```
  - It must not receive `-coverprofile`, `-coverpkg`, quality-scope tags, quality-scope excludes, or test-file-pattern mutation excludes.

## BDD Feature-To-Command Coverage Matrix

BDD must assert the same exact command contract with literal argv blocks. Helper code may normalize artifact placeholders and tool resolver form, but must not derive expected args from production `gatecheck`.

Canonical comparison rule:

- For Go toolchain commands (`go list`, `go test`, `go tool cover`, `go list -m -json`), compare the full argv exactly after replacing artifact paths and package inventory format with their documented placeholders.
- For external tools resolved locally or through `go run`, normalize only the executable prefix:
  - local `gremlins` and `go run <gremlinsSpec>` compare from `unleash` onward;
  - local `gocyclo` and `go run <gocycloSpec>` compare from `-over` onward.
- After this prefix normalization, every remaining arg MUST match exactly and in order.

Required BDD command coverage:

| Command contract | Feature(s) that MUST assert exact argv |
| --- | --- |
| `QualityScopeInventory` `go list` | `features/quality_scope_inventory.feature`, plus scoped examples in `features/coverage.feature`, `features/crap.feature`, and `features/mutation_runner_scope.feature` |
| `CoveredTest` `go test` | `features/coveredtest.feature`, `features/coverage.feature`, `features/crap.feature` |
| `MutationRunner.Scan` | `features/mutation_runner_scope.feature`, and scan-producing examples in `features/mutationcoverage.feature` and `features/mutationsites.feature` when those features dispatch scan |
| `MutationRunner.Kill` / `MutationKills` | `features/mutationkills.feature` |
| `Coverage` `go tool cover` | `features/coverage.feature`, `features/crap.feature` |
| `CRAP` gocyclo | `features/crap.feature` |
| `CRAP` module metadata | `features/crap.feature` if command logging can observe it; otherwise a harness unit test must assert `go list -m -json` exactly |
| `Duration` no subprocess | `features/duration.feature` if present, otherwise `internal/harness/step_duration_test.go` must assert no command dispatch |
| Integration `go test` | magefile flow/integration tests must assert exact argv; if no BDD feature exists, `magefiles/magefile_flow_test.go` must cover it |

- `features/quality_scope_inventory.feature`
  - Verifies `QualityScopeInventory` exact `go list` argv.
  - Must include cases proving tags appear as `-tags=<csv>` and excludes/test-file patterns do not affect inventory args.

- `features/coveredtest.feature`
  - Verifies `CoveredTest` exact `go test` argv for scoped `-coverpkg`, quality-scope tags, and `-count=1`.
  - Must include a case proving test-file patterns do not add extra `go test` args or change `-coverpkg`.

- `features/coverage.feature`
  - Verifies exact `go tool cover -func=<artifact>/coverage-filtered.out` when exclude segments or test-file patterns are active.
  - Verifies exact `go tool cover -func=<artifact>/coverage.out` when no file-level filters are active.

- `features/crap.feature`
  - Verifies exact gocyclo target directory argv for scoped package dirs.
  - Verifies exact CRAP cover-function command uses `coverage-filtered.out` when filters are active.
  - Verifies test-file patterns affect parsing/filtering, not gocyclo target argv.

- `features/mutation_runner_scope.feature`
  - Verifies exact `MutationRunner.Scan` argv for package seed, package/path excludes, overlapping excludes, tags, dry-run, root files, metacharacter files, slash normalization, and all-excluded no-dispatch.
  - Must replace `test file patterns become concrete exclude-file arguments` with a scenario that creates at least two test files in one in-scope package and one test file in a second in-scope package. Expected argv must contain exactly one package-level regex per in-scope package and must not contain any per-file test regex.
  - Required expected snippet:
    ```text
    --exclude-files=^internal/app/.*_test\.go$
    --exclude-files=^vendor/lib/.*_test\.go$
    ```
  - Required negative assertions:
    ```text
    --exclude-files=^internal/app/foo_test\.go$
    --exclude-files=^internal/app/bar_test\.go$
    --exclude-files=^vendor/lib/lib_test\.go$
    ```
    must not appear.
  - Must include an excluded-package case proving `quality scope excludes "testutil"` plus `*_test.go` emits `^internal/testutil(/|$)` and does not also emit `^internal/testutil/.*_test\.go$`.

- `features/mutationkills.feature`
  - Verifies exact full-run mutation argv has the same coverpkg and exclude regex contract as scan, without `--dry-run`.
  - Must include the same minimal test-pattern regex expectation for full-run mutation when test-file patterns are configured.

- `features/mutationcoverage.feature`, `features/mutationsites.feature`, and `features/mutationkills.feature`
  - Verify metric consumers use the same parsed `ExcludeSegments` and `TestFilePatterns` as command producers.
  - These features do not assert new subprocess argv except where they invoke scan/kill; they must assert scoped evaluation excludes the same files represented by the minimal regex command contract.

## Anti-Bullshit Requirements

These are explicit failure conditions for implementation and review:

- Any dogfood mutation scan with more test-file `--exclude-files` flags than in-scope package dirs fails the plan.
- For pattern-only test exclusions, emitted test-pattern exclude count MUST equal the number of otherwise in-scope package dirs that contain matching test or xtest files. It MUST NOT equal the number of matching files. Excluded package dirs do not count because their directory regex already covers all children.
- Any BDD expected argv that names a specific `*_test.go` file for a test-file-pattern-only exclusion fails the plan, unless the scenario is specifically about an explicit file path exclusion.
- Any implementation that preserves per-file test regexes and only sorts/deduplicates them fails the plan.
- Any command producer that reparses raw quality-scope strings instead of consuming `QualityScopeCommandScope` projections fails the plan.
- Any trace or verification helper that creates `.gotmp` repo artifacts, requires `.gitignore` changes, or changes measured coverage/CRAP fails the plan.
- Any subagent output accepted without parent-agent review against this plan, the old plan, the intent doc, and repo evidence fails the plan.
- Any implementation that lacks exact dogfood argv evidence after the fix fails the plan.
- Any implementation that leaves `.gotmp` untracked noise in `git status` fails the plan.

## Design Shape

The canonical implementation remains in `internal/gatecheck`:

- `internal/gatecheck/quality_scope_command_scope.go` stays the public internal projection surface.
- `internal/gatecheck/gremlins_exclude.go` owns regex minimization and serialization.
- Test-file-pattern argv minimization should operate over filtered, in-scope package rows, not all inventory rows. Excluded package dirs are already covered by directory regexes and must not add redundant test-pattern regexes.
- The minimal regex for a package directory should be anchored to the canonical root-relative package dir and match test basenames selected by `TestFilePatterns`. For the repo’s `*_test.go`, this should produce package-level regexes such as `^gate/.*_test\.go$` rather than `^gate/foo_test\.go$` per file.
- For root package dirs (`.`), the regex must remain anchored to root-level files only, not every nested package, unless the pattern is explicitly meant to cross directories. Existing basename semantics should be preserved.

## BDD And Unit Contract Updates

Update actor-visible expectations before implementation:

- In `features/mutation_runner_scope.feature`, rename/rewrite the test-file-pattern scenario to assert minimal package regex behavior, not concrete per-file behavior.
- Keep existing BDD scenarios for root module files, metacharacter-bearing paths, path-segment source files outside package inventory, overlapping excludes, and all-excluded failure.
- Add or update unit tests in `internal/gatecheck/gremlins_exclude_test.go` and `internal/gatecheck/quality_scope_command_scope_test.go` proving:
  - Multiple test files in the same in-scope package emit one package regex.
  - Multiple in-scope packages emit one regex per package dir.
  - Excluded packages do not emit redundant test-pattern regexes.
  - Root package behavior does not accidentally match nested packages.
  - File-specific excludes still emit concrete file regexes when a wider regex would remove in-scope code.
- Update parity tests in `internal/gatecheck/quality_scope_command_scope_parity_test.go`, `internal/harness/quality_scope_command_scope_parity_test.go`, and `gate/quality_scope_inventory_consumer_test.go` to expect minimal regexes.

## Mandatory Subagent Workflow

Subagents are required for execution. They are not authoritative; their output is untrusted draft work until reviewed and reconciled by the parent agent.

Before launching any subagent, build an execution-context packet from:

- `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md`
- `.cursor/plans/quality-scope-minimal-regex_df54ad1f.plan.md`
- `docs/mage-gate-intent-and-design.md`
- `docs/kb/coding-standards.md`
- `docs/kb/verification.md`
- `docs/kb/issue-ownership.md`
- `docs/kb/agent-delegation.md`
- `docs/kb/code-smells.md`
- `docs/kb/error-handling.md`
- Current failures or trace evidence showing per-file test regex enumeration

Every subagent prompt must include:

- Exact task scope and owned files.
- The command contract and anti-bullshit requirements from this plan.
- No public API changes, no shims, no parallel translators, no helper-derived BDD expected args.
- The instruction: “Fully implement. Do not simplify.”
- Required targeted tests for that subagent’s scope.
- Required final response: changed files, exact tests run, remaining risks, and any plan conflicts found.

Required subagents:

- `task-executor` for BDD contract updates only.
  - Owns `features/*.feature` and existing BDD assertion plumbing only if needed.
  - Must not touch production code.
  - Must make the old concrete-per-file expectation fail and the new exact minimal-regex expectation actor-visible.

- `task-executor` for `internal/gatecheck` minimizer and command-scope projection tests.
  - Owns `internal/gatecheck/gremlins_exclude.go`, `internal/gatecheck/quality_scope_command_scope.go` only if projection interfaces require it, and related `internal/gatecheck/*_test.go`.
  - Must not touch harness or public `gate` files.

- `task-executor` for harness/public parity rewiring.
  - Owns `internal/harness` parity tests and `gate` inventory consumer tests.
  - Must not change public APIs or `Duration`.

- `task-executor` or `shell` subagent for verification and `.gotmp` cleanup.
  - Owns test/gate execution, exact argv capture, `git status` evidence, and cleanup evidence.
  - Must not make production code edits.

- `critical-reviewer` after BDD and gatecheck phases.
  - Reviews plan compliance before harness/public parity updates proceed.

- `critical-reviewer` after all implementation and verification.
  - Performs final adversarial review against the old plan, this completion plan, and `docs/mage-gate-intent-and-design.md`.

Subagent output review requirements:

- Parent agent must read the full subagent output, not only the summary.
- Parent agent must check each output against its prompt, this plan, the old plan, and the intent doc.
- Parent agent must treat review findings as hypotheses until reconciled with repo-local tests, lint, static analysis, and command argv evidence.
- Parent agent must check for scope reduction, missing tests, helper-derived expected args, public API drift, duplicated translators, and incomplete cleanup.
- If a subagent is wrong or incomplete, re-delegate with a corrective prompt. Maximum two re-delegations per task; after that split the task or stop with the concrete blocker.
- Parent agent must not claim completion based solely on subagent success.
- Parent agent must reproduce or independently verify test/lint/argv evidence before accepting a subagent claim of success.

## Local Artifact Constraint

This plan must not introduce any permanent local-artifact policy change.

- Do not add `.gotmp/` or any equivalent trace/debug path to `.gitignore`.
- Do not commit trace/debug helpers.
- Do not verify by copying trace helpers into measured source directories such as `magefiles/`.
- Do not create new `.gotmp` repo artifacts during implementation or verification.
- Exact argv verification must use existing tests, existing command logs already captured before this plan, or a tracing approach outside the repo tree that does not affect coverage, CRAP, lint, or `git status`.

## Verification

Run verification in this order:

- Targeted BDD for `features/mutation_runner_scope.feature` first, proving the old concrete-per-file expectation fails before implementation and the new exact minimal-regex expectation passes after.
- Targeted BDD for `features/mutationkills.feature`, `features/mutationcoverage.feature`, `features/mutationsites.feature`, `features/coveredtest.feature`, `features/coverage.feature`, `features/crap.feature`, and `features/quality_scope_inventory.feature`, proving every exact command contract above has actor-visible coverage.
- Targeted Go tests for `internal/gatecheck`, `internal/harness`, and `gate` inventory consumers.
- Lint/static analysis.
- Full `mage gate` with normal `coverage_min = 90.0` and `crap_max = 8` restored.
- Capture or assert exact dogfood command argv after implementation without adding measured source files under `magefiles/`, without creating `.gotmp`, and without temporary threshold changes.
- Confirm `git status` has no `.gotmp` artifacts, no trace/debug helpers, no `.gitignore` change for local artifacts, and no temporary threshold changes.
- Run final `critical-reviewer` and reconcile every finding with evidence.
- Required evidence artifacts in final report:
  - Targeted BDD command output for every feature listed in the coverage matrix.
  - Targeted Go test command output for `internal/gatecheck`, `internal/harness`, `gate`, and `magefiles` command-flow tests.
  - Lint/static analysis command output.
  - Full `mage gate` command output.
  - Exact dogfood argv log or equivalent test assertion output showing scan and kill minimal regex argv.
  - `git status --short` output proving `.gotmp`, trace/debug helper files, `.gitignore` local-artifact changes, and temporary threshold edits are absent.
- Perform a final parent-agent review against `docs/mage-gate-intent-and-design.md` and `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md`. This review must explicitly answer:
  - Does `QualityScope` remain the single measurement statement?
  - Does every inventory-consuming step use one shared command command-scope projection?
  - Are gremlins exclude args minimal regexes rather than file enumeration?
  - Do coverage, CRAP, mutation scan/kill, `MutationSites`, and `MutationCoverage` use identical parsed filters?
  - Did `Duration` remain outside command-scope parity?
  - Are exact dogfood args and exact BDD argv contracts aligned?
  - Is `.gotmp` absent from tracked/untracked status with no `.gitignore` local-artifact change?

## Expected Outcome

Gremlins argv for the repo dogfood scope should shrink from 100+ test-file excludes to a small set of regexes: directory regexes for excluded packages/paths, concrete file regexes only for truly file-specific non-package exclusions, and one test-pattern regex per in-scope package directory. Coverage, CRAP, mutation scan/kill, mutation sites, and mutation coverage should continue to use identical parsed quality-scope filters.
