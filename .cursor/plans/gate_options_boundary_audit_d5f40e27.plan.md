---
name: gate-options-boundary-audit
overview: "SUPERSEDED by issue-21-complete-fix_76fd8a79.plan.md. This was a partial 7-priority slice, not the complete issue #21 fix."
todos:
  - id: p1-quality-scope-options
    content: Refactor QualityScopeOption to mutate private qualityScopeConfig; store slices not CSV; update command-scope bridge to accept []string excludes
    status: in_progress
  - id: p2-inventory-fingerprint
    content: Add full QualityScope fingerprint to QualityScopeInventoryOutput; validate in consuming steps; rewrite 3 cross-scope unit tests to expect rejection
    status: pending
  - id: p3-duration-decouple
    content: Remove QualityScope param from Duration; remove filtering logic from harness StepDuration; delete BDD scenario; update all call sites
    status: pending
  - id: p4-mutation-runner-failfast
    content: Change NewMutationRunner to (MutationRunner, error); reject nil deps at construction; update all call sites and nil-dep tests
    status: pending
  - id: p5-silent-defaults
    content: Remove gateRoot empty-string default; error on empty root at step entry
    status: pending
  - id: p6-pattern-validation
    content: Trim whitespace and reject whitespace-only in validateGoTestPackagePattern
    status: pending
  - id: p7-document-decisions
    content: Document accepted compromises at type level (zero-value runtime checks, threshold set flags, public option pattern)
    status: pending
isProject: true
---

# Gate Strict-Boundary Audit (Issue #21)

## Execution Model

Each priority is a sub-agent task. After each priority completes, a reviewer sub-agent validates the change before proceeding. Verification at every step: targeted `go test`, lint, and compile.

## Priority 1: QualityScope Option Isolation + Typed Storage

**No public API signature change.** Internal refactor only.

### Change description

1. Replace `type QualityScopeOption func(*QualityScope)` with `type QualityScopeOption func(*qualityScopeConfig)` where `qualityScopeConfig` is private and excludes `packages`.
2. Store `tags`, `excludeSegments`, `testFilePatterns` as `[]string` slices in `QualityScope` instead of comma-joined strings.
3. Remove `splitQualityScopeList` and `appendQualityScopeValues` helpers.
4. Getters (`Tags()`, `ExcludeSegments()`, `TestFilePatterns()`) return the stored slices directly (nil if empty).
5. Update `qualityScopeCommandScope()` in `gate/steps_quality_scope_inventory.go` to pass `qs.ExcludeSegments()` (now `[]string`) instead of `qs.excludeSegments` (was raw CSV string).
6. Update `NewQualityScopeCommandScope` signature to accept `excludeSegments []string` instead of `rawExcludeSegments string`; remove `ParseExcludeSegments` call at construction (already parsed).
7. Update `StepQualityScopeInventory` in harness: pass `strings.Join(qs.Tags(), ",")` or equivalent at the boundary (currently passes raw `qs.tags` CSV — functionally identical after refactor).

### Files touched

- `gate/quality_scope.go` — type + constructor + options + storage
- `gate/steps_quality_scope_inventory.go` — bridge function
- `internal/gatecheck/quality_scope_command_scope.go` — constructor signature
- `internal/gatecheck/quality_scope_command_scope_test.go` — update callers of `NewQualityScopeCommandScope`

### Verification

- `go test ./gate/... ./internal/gatecheck/...` passes
- `golangci-lint run ./gate/... ./internal/gatecheck/...` passes
- No BDD changes needed (option names and behavior unchanged)

---

## Priority 2: Inventory Scope Fingerprint

**No public API signature change.** Adds runtime validation that rejects inventory/scope mismatch.

### Change description

1. Add `scopeFingerprint string` field to `QualityScopeInventoryOutput`.
2. Compute fingerprint at production time in `QualityScopeInventory()`: deterministic hash of `(packages, sorted(tags), sorted(excludeSegments), sorted(testFilePatterns))`.
3. Add `qualityScopeFingerprint(QualityScope) string` helper — SHA-256 of canonical string `packages|tag1,tag2|excl1,excl2|pat1,pat2` (sorted slices, pipe-delimited fields). Returns hex string.
4. In `requireQualityScopeInventory`, add `validateInventoryScopeMatch(inventory, qualityScope)` that compares `inventory.scopeFingerprint` against `qualityScopeFingerprint(qualityScope)`. Return `ErrQualityScopeInventoryMismatch` on failure.
5. Reuse existing `ErrQualityScopeInventoryMismatch` sentinel (already exists for store/root mismatch — scope mismatch is the same category).

### Unit tests that must be rewritten

These 3 tests currently pass inventory from scope A to consumers with scope B. They must be rewritten to **expect rejection**:

- `TestCoveredTestAcceptsInventoryWithDifferentConsumerDiscoveryInputs` — rename to `TestCoveredTestRejectsInventoryWithDifferentScope`; assert `ErrQualityScopeInventoryMismatch`.
- `TestCoveredTestReusesInventoryWithConsumerExcludeFilter` — rename to `TestCoveredTestRejectsInventoryWithDifferentExcludes`; assert error on the `filteredScope` call (second CoveredTest).
- `TestMutationScanReusesInventoryWithConsumerTestFilePatterns` — rename to `TestMutationScanRejectsInventoryWithDifferentTestFilePatterns`; assert error on the `filteredScope` call.

### Files touched

- `gate/step.go` — add `scopeFingerprint` field
- `gate/quality_scope.go` — add `qualityScopeFingerprint` helper
- `gate/steps_quality_scope_inventory.go` — compute fingerprint at production; validate at consumption
- `gate/quality_scope_inventory_consumer_test.go` — rewrite 3 tests

### Verification

- `go test ./gate/...` passes (rewritten tests assert rejection)
- BDD unchanged (all BDD uses same scope for inventory and consumers)

---

## Priority 3: Duration Decoupled from QualityScope

**PUBLIC API BREAKING CHANGE.** Removes `qualityScope QualityScope` parameter.

### Public API diff

```diff
 func Duration(
 	ctx context.Context,
 	runner CommandRunner,
 	store *ArtifactStore,
 	fileOps FileOps,
 	root string,
 	testOutput TestOutput,
-	qualityScope QualityScope,
 	maxSeconds DurationThreshold,
 ) (err error) {
```

### Harness API diff

```diff
 func (h *StepRunner) StepDuration(
 	_ context.Context,
 	unitMaxSeconds float64,
-	upstreamStepID, excludeSegments string,
+	upstreamStepID string,
 ) error {
```

### Internal logic removal

In `internal/harness/step_duration.go`, `loadFilteredTestDurations` drops the `excludeSegments` parameter and the `gatecheck.FilterTestDurations` call. All parsed test events are checked — no filtering.

```diff
-func (h *StepRunner) loadFilteredTestDurations(
-	upstreamStepID, excludeSegments string,
-) ([]gatecheck.TestDuration, error) {
+func (h *StepRunner) loadTestDurations(
+	upstreamStepID string,
+) ([]gatecheck.TestDuration, error) {
 	eventsData, err := h.store.Read(upstreamStepID, "test-events.jsonl")
 	if err != nil {
 		return nil, fmt.Errorf("%w: read test events from store: %w", ErrDurationFailed, err)
 	}
 	tests, err := gatecheck.ParseTestEvents(bytes.NewReader(eventsData))
 	if err != nil {
 		return nil, fmt.Errorf("%w: parse test events: %w", ErrDurationFailed, err)
 	}
-	excludes := gatecheck.ParseExcludeSegments(excludeSegments)
-	filtered := gatecheck.FilterTestDurations(tests, excludes)
-	if filtered == nil {
-		return nil, fmt.Errorf("%w: %w", ErrDurationFailed, gatecheck.ErrAllPackagesExcluded)
-	}
-	return filtered, nil
+	if len(tests) == 0 {
+		return nil, fmt.Errorf("%w: no test events found", ErrDurationFailed)
+	}
+	return tests, nil
 }
```

### Gate-level logic removal

In `gate/steps_crap_duration.go`:

```diff
 func Duration(
 	ctx context.Context,
 	runner CommandRunner,
 	store *ArtifactStore,
 	fileOps FileOps,
 	root string,
 	testOutput TestOutput,
-	qualityScope QualityScope,
 	maxSeconds DurationThreshold,
 ) (err error) {
 	emitStepStart(runner, stepLineDuration, testOutput.qualifier)
 	if checkErr := validateMaxSeconds(maxSeconds); checkErr != nil {
 		return checkErr
 	}
 	if checkErr := requireStoreDeps(runner, fileOps, store); checkErr != nil {
 		return checkErr
 	}
 	if checkErr := validateTestOutputToken(testOutput); checkErr != nil {
 		return checkErr
 	}
-	if checkErr := validateQualityScope(qualityScope); checkErr != nil {
-		return checkErr
-	}
 	if checkErr := requireUpstreamArtifact(store, testOutput.stepID, "test-events.jsonl"); checkErr != nil {
 		return checkErr
 	}
 	id := nextID("duration")
 	harn, err := harness.NewStepRunner(
 		gateRoot(root), "", testOutput.scope.Packages(), runner, fileOps, store, id,
 	)
 	if err != nil {
 		return fmt.Errorf("create harness: %w", err)
 	}
 	defer func() { err = errors.Join(err, wrapHarnessCleanup("duration", runner, harn.Cleanup())) }()
 	return wrapStepError(
 		"duration",
 		runner,
 		harn.StepDuration(
 			ctx,
 			maxSeconds.maxSeconds,
 			testOutput.stepID,
-			qualityScope.excludeSegments,
 		),
 	)
 }
```

### BDD diff — `features/duration.feature`

```diff
-  Scenario: quality scope excludes narrow the duration boundary
-    Given the codebase has 95% test coverage
-    And the quality scope excludes "testutil"
-    And the output mode is agent
-    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
-    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
-    When the gate runs steps qualityscopeinventory, coveredtest, duration
-    Then the step passes
-    And the command `go` is run with arguments:
-      """
-      test
-      ./zz_bdd_not_default/...
-      -json
-      -coverprofile=<artifact>/coverage.out
-      -coverpkg=example.com/mod/zz_bdd_not_default/pkg
-      -count=1
-      """
```

This scenario is deleted entirely. Duration checks all test events from the run. If the consumer wants different duration thresholds for integration vs unit tests, they call Duration separately on separate TestOutputs with separate thresholds.

### BDD step definition diff — `features/internal/steps/step_execution.go`

```diff
 func (s *scenarioState) runDuration(...) error {
-	return qg.Duration(ctx, display, store, mem, root, testOut, s.qualityScope, threshold)
+	return qg.Duration(ctx, display, store, mem, root, testOut, threshold)
 }
```

(All 3 Duration call paths in this function updated — remove `s.qualityScope` / `qg.QualityScope{}`.)

### Production call site diffs

`magefiles/magefile.go:151`:
```diff
-	return qg.Duration(
-		ctx, runner, store, fileOps, root, testRun, qualityScope, durationMax(cfg),
-	)
+	return qg.Duration(
+		ctx, runner, store, fileOps, root, testRun, durationMax(cfg),
+	)
```

`magefiles/magefile_runtime_steps.go:253`:
```diff
-	return qg.Duration(
-		ctx, runner, store, fileOps, repoRoot, testRun, scope, durationMax(&cfg),
-	)
+	return qg.Duration(
+		ctx, runner, store, fileOps, repoRoot, testRun, durationMax(&cfg),
+	)
```

### Test call site changes (all in `gate/`)

Remove the `qualityScope`/`scope`/`QualityScope{}` argument from every `Duration(...)` call in:
- `gate/validation_test.go` (4 calls)
- `gate/steps_test.go` (4 calls)
- `gate/steps_precondition_test.go` (2 calls)
- `gate/steps_duration_test.go` (1 call)
- `gate/steps_cleanup_test.go` (1 call)
- `gate/step_start_output_contract_test.go` (1 call)
- `gate/acceptance_output_test.go` (2 calls)
- `integration/examplecheck/integration_test_contracts_test.go` (1 call)
- `integration/examplecheck/integration_test_contracts_helpers_test.go` (1 call)

### Also delete

- `gatecheck.FilterTestDurations` function and its tests — delete entirely
- `gatecheck.ErrAllPackagesExcluded` — grep first; delete if only used by the duration filter path; keep if used by mutation/coverage steps

### Verification

- `go build ./...` compiles
- `go test ./gate/... ./internal/harness/... ./internal/gatecheck/...` passes
- BDD: `go test ./features/...` passes (scenario removed)
- `golangci-lint run` passes

---

## Priority 4: Fail-Fast MutationRunner

**PUBLIC API BREAKING CHANGE.** Changes return type.

### Public API diff

```diff
 func NewMutationRunner(
 	runner CommandRunner,
 	resolver ToolResolver,
 	store *ArtifactStore,
 	fileOps FileOps,
-) MutationRunner {
-	return &productionMutationRunner{
-		runner:   runner,
-		resolver: resolver,
-		store:    store,
-		fileOps:  fileOps,
-	}
+) (MutationRunner, error) {
+	if runner == nil {
+		return nil, fmt.Errorf("%w: runner", ErrNilDependency)
+	}
+	if resolver == nil {
+		return nil, fmt.Errorf("%w: resolver", ErrNilDependency)
+	}
+	if store == nil {
+		return nil, fmt.Errorf("%w: store", ErrNilDependency)
+	}
+	if fileOps == nil {
+		return nil, fmt.Errorf("%w: fileOps", ErrNilDependency)
+	}
+	return &productionMutationRunner{
+		runner:   runner,
+		resolver: resolver,
+		store:    store,
+		fileOps:  fileOps,
+	}, nil
 }
```

### Call site migration pattern

Production (`magefiles/magefile_mutations.go`, 3 sites):
```diff
-	mr := qg.NewMutationRunner(runner, resolver, store, fileOps)
+	mr, err := qg.NewMutationRunner(runner, resolver, store, fileOps)
+	if err != nil {
+		return err
+	}
```

BDD (`features/internal/steps/scenario_mutation_scan.go`, `step_execution.go`):
```diff
-	mr := qg.NewMutationRunner(display, resolver, store, mem)
+	mr, err := qg.NewMutationRunner(display, resolver, store, mem)
+	if err != nil {
+		return err
+	}
```

### Test migration — nil-dep tests (`gate/mutation_runner_test.go`)

Tests that currently pass nil to `NewMutationRunner` and assert error from `.Scan`/`.Kill` must change to assert error from `NewMutationRunner` itself:

```diff
-	_, e := NewMutationRunner(nil, fakeResolver, store, mem).Scan(...)
-	requireErrorIs(t, e, ErrNilDependency)
+	_, e := NewMutationRunner(nil, fakeResolver, store, mem)
+	requireErrorIs(t, e, ErrNilDependency)
```

Tests with valid deps (majority) add error check:
```diff
-	mr := NewMutationRunner(runner, resolver, store, mem)
+	mr, err := NewMutationRunner(runner, resolver, store, mem)
+	if err != nil {
+		t.Fatalf("NewMutationRunner: %v", err)
+	}
```

Remove nil-dep validation from `Scan` and `Kill` methods (`mutationValidateBeforeScan` etc.) since it is now impossible to reach those paths.

### Verification

- `go build ./...` compiles
- `go test ./gate/... ./features/... ./magefiles/... ./integration/...` passes
- Nil-dep tests now assert construction-time rejection

---

## Priority 5: Remove gateRoot Silent Default

**No public API signature change.** Behavioral change: empty `root` now returns an error instead of silently becoming `"."`.

### Change

```diff
 func gateRoot(root string) string {
-	if root == "" {
-		return defaultRoot
-	}
-	return root
+	return root
 }
```

Add `validateRoot(root string) error` that returns `fmt.Errorf("%w: root", ErrMissingValue)` on empty/whitespace-only input. Call from every public step that accepts `root`:

`Test`, `CoveredTest`, `Coverage`, `Crap`, `Duration`, `Lint`, `Compile`, `Vet`, `Deadcode`, `MarkdownLint`, `Format`, `QualityScopeInventory`, `MutationRunner.Scan`, `MutationRunner.Kill`, `MutationKills`.

No callers pass empty root today, so no call sites break.

### Files touched

- `gate/steps_public.go` — add `validateRoot`; add call to each step's validation block
- `gate/steps_crap_duration.go` — add call in Crap and Duration
- `gate/steps_coverage.go` — add call in Coverage
- `gate/steps_quality_scope_inventory.go` — add call in QualityScopeInventory
- `gate/mutation_runner.go` — add call in Scan and Kill
- `gate/mutation_kills_public.go` — add call in MutationKills

### Verification

- `go test ./gate/...` passes
- Add unit test table: each step rejects empty root with `ErrMissingValue` (extend existing `steps_precondition_test.go` pattern)

---

## Priority 6: Pattern Validation Tightening

**No public API signature change.** Behavioral tightening.

### Change — `gate/pattern_validate.go`

```diff
 func validateGoTestPackagePattern(pattern string, ifEmpty error) error {
+	pattern = strings.TrimSpace(pattern)
 	if pattern == "" {
 		return ifEmpty
 	}
 	if strings.HasPrefix(pattern, "-") {
 		return fmt.Errorf("%w: %q looks like a flag, not a package pattern", ErrInvalidOption, pattern)
 	}
 	return nil
 }
```

Note: `NewQualityScope` and `NewPackageScope` will now reject `"  "` (whitespace-only). This is a correctness fix, not an API change.

### Verification

- Add unit tests: whitespace-only patterns rejected
- Existing tests pass (none use whitespace-only patterns)

---

## Priority 7: Document Accepted Compromises

Add doc comments to these types acknowledging Go zero-value limitation:

- `CoverageThreshold`, `CrapThreshold`, `DurationThreshold`, `MutationSitesThreshold`, `MinKillRateThreshold`, `MutationCoverageThreshold` — add `// Zero value is invalid; construct via [MinPercent], [MaxScore], etc.`
- `PackageScope` — add `// Zero value is invalid; construct via [NewPackageScope].`
- `QualityScope` — add `// Zero value is invalid; construct via [NewQualityScope].`
- `QualityScopeOption` type doc — add note: "Typed option functions exist for scope modifiers that carry invariants (tags, excludes, test-file patterns). Generic tool argv uses *Args constructors."

### Files touched

- `gate/options.go` — threshold type docs
- `gate/package_scope.go` — PackageScope doc
- `gate/quality_scope.go` — QualityScope and QualityScopeOption docs

### Verification

- `golangci-lint run` passes (no code changes, only comments)
- `go doc ./gate` output reviewed for accuracy

---

## Sub-Agent Orchestration

Each priority is executed by a `task-executor` sub-agent with:
- Full context from this plan
- Targeted file list
- Explicit verification commands

After each priority, a `critical-reviewer` sub-agent validates:
- No unintended API changes leaked
- Tests cover the change
- No regressions in compile/lint/test

Final verification after all priorities: full `mage gate`.
