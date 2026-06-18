---
name: quality-scope-global-minimizer
overview: "Tighten the quality-scope mutation regex plan so gremlins exclude argv is minimized after combining all actual inputs: package inventory rows, walked root-relative source files, exclude segments, and test-file patterns. This complements the existing command-scope and minimal-regex plans by closing the unlisted-source/file-enumeration gap that current BDD still blesses."
todos:
  - id: reconcile-plans
    content: "Record the compatibility decision: old plans remain authoritative except stale unlisted-source enumeration expectations are superseded by global minimization."
    status: completed
  - id: tighten-bdd
    content: Update mutation BDD to assert globally minimized segment regexes for walked-only excluded source trees.
    status: completed
  - id: update-tests
    content: Replace unit and parity expectations that currently bless per-file unlisted-source excludes.
    status: completed
  - id: implement-minimizer
    content: Move gremlins exclude serialization to collect all actual inputs, globally minimize, then serialize.
    status: completed
  - id: verify-dogfood
    content: Prove exact dogfood scan/kill argv has directory regexes for integration/testdata and no enumerated files.
    status: completed
isProject: false
---

# Quality Scope Global Minimizer Plan

## Plan Compatibility

The existing plans are compatible with the user requirement at their top-level contract:

- `.cursor/plans/quality-scope-command-plan_758ffbeb.plan.md` says one `QualityScopeInventory` plus one `QualityScope` produces one internal command scope, and minimal means directory regexes where a directory excludes all children and file regexes only where wider exclusion would remove in-scope code.
- `.cursor/plans/quality-scope-minimal-regex_df54ad1f.plan.md` says dogfood gremlins argv must include `^integration(/|$)` and `^testdata(/|$)`, must not enumerate files under `testdata`, and must compare exact dogfood argv.

The contradiction is not in the user requirement. It is in stale acceptance text carried forward from the narrower test-pattern fix:

- `quality-scope-command-plan_758ffbeb.plan.md` preserves the BDD scenario `source files outside package inventory are excluded only by quality-scope segments` as-is.
- `quality-scope-minimal-regex_df54ad1f.plan.md` says to keep BDD scenarios for source-path files outside package inventory, while also requiring dogfood `^testdata(/|$)`. This must be clarified: keep the scenario category, but replace its expected argv with globally minimized segment regexes when safe.
- The current BDD and unit tests expect `^testdata/failures/calc\.go$`; those expectations contradict the intended final-list minimization.

## Required Design Tightening

Define mutation exclude projection as a two-stage operation in `internal/gatecheck`:

1. Collect all exclusion facts from actual inputs:
   - package inventory rows from `QualityScopeInventory`
   - walked root-relative `.go` source files needed by gremlins
   - parsed exclude segments
   - parsed test-file patterns
2. Minimize the completed exclusion set before serialization:
   - Emit a segment directory regex such as `^testdata(/|$)` when that exclude segment covers every matching source path under the segment and does not remove any in-scope production file outside the quality-scope exclusion.
   - Emit package-level test-pattern regexes such as `^gate/.*_test\.go$` for in-scope packages containing matching test files.
   - Emit concrete file regexes only for genuinely file-specific exclusions, including root module files, explicit file path excludes, metacharacter-bearing explicit paths, or cases where a wider directory regex would exclude in-scope production code.
   - Sort final `--exclude-files=` flags lexicographically after minimization.

This closes the current piecemeal behavior where package-row matches produce directory regexes but walked-only source matches produce per-file regexes.

## BDD Contract Updates

Update BDD before production changes:

- In `features/mutation_runner_scope.feature`, replace the `source files outside package inventory are excluded only by quality-scope segments` expected argv so `testdata` emits `--exclude-files=^testdata(/|$)`, not `--exclude-files=^testdata/failures/calc\.go$`.
- Add a second unlisted-source scenario with multiple files under one excluded segment, proving the command emits one segment regex and no per-file regexes.
- Add a mixed-safety scenario proving a genuinely file-specific explicit path still emits a concrete escaped file regex when a directory regex would be too broad.
- Add dogfood-shaped BDD or command-flow coverage for `cmdtest`, `features`, `gatetest`, `integration`, and `testdata` together, using real-shaped source inventory where `integration` and `testdata` are walked sources but absent from package inventory.

BDD expected argv must remain literal actor-level expectations. BDD helpers may normalize artifact paths and tool resolver prefixes only; they must not call or mirror `internal/gatecheck` minimizer code.

## Unit And Parity Updates

Update tests that currently bless enumeration:

- In `internal/gatecheck/quality_scope_command_scope_test.go`, change `excludes_source_outside_packages` to expect `^testdata(/|$)` when the segment safely covers the walked source subtree.
- In `internal/gatecheck/gremlins_exclude_test.go`, add coverage for real dogfood shape: no synthetic `testdata` package row, walked source files under `testdata` and `integration`, and expected directory regexes.
- In `internal/harness/step_mutation_scope_test.go`, update unlisted testdata expectations to the segment directory regex.
- Add a negative assertion that dogfood-shaped argv does not contain any `^testdata/...\.go$` or `^integration/...\.go$` per-file excludes.
- Keep existing tests for root files, slash normalization, metacharacter escaping, all-excluded failure, and package-level test-pattern regexes.

## Implementation Boundary

Keep implementation ownership in `internal/gatecheck`:

- `internal/gatecheck/quality_scope_command_scope.go` remains the shared projection surface.
- `internal/gatecheck/gremlins_exclude.go` owns collection and global minimization.
- `internal/gatecheck/gremlins_exclude_flags.go` owns serialization only.
- `internal/harness/step_mutation.go` continues to gather source inventory and call the command-scope projection; it must not decide minimization.

Do not change public `gate` APIs. Do not add compatibility shims, feature flags, or a second translator.

## Verification

Run verification in this order:

- Targeted BDD for `features/mutation_runner_scope.feature`, first showing the old per-file unlisted-source expectation fails for the new contract, then passes after implementation.
- Targeted BDD for `features/mutationkills.feature`, `features/mutationcoverage.feature`, and `features/mutationsites.feature` where they dispatch or consume mutation scope.
- Targeted Go tests for `internal/gatecheck`, `internal/harness`, `gate`, and `magefiles` command-flow coverage.
- Lint/static analysis using repo-local config.
- Exact dogfood argv assertion or capture proving scan and kill contain `^integration(/|$)` and `^testdata(/|$)` and do not enumerate files under those segments.
- Full `mage gate` with normal thresholds.
- `git status --short` proving no trace helpers, `.gotmp`, `.gitignore` local-artifact changes, or threshold edits remain.

[Standards:Checked]