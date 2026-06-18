# Mage-gate: intent and design

This document states **why this exists**, what it promises, and where the boundaries are. It is not API documentation or implementation rationale - those live in godoc, the README, and code comments.

---

## 1. Why this exists

Build tools produce output optimized for human readers. They celebrate near-completion ("235 of 237 tests passed!"), emit thousands of verbose log lines, and bury actionable signals in noise. Humans compensate - they know what they changed, they scan visually, they skip the irrelevant.

Agents cannot do this. An agent facing thousands of lines of `go test` output has no reliable way to find the three lines that matter. Worse, success noise triggers satisficing - the agent sees "almost passing" and optimizes for cosmetic fixes instead of root causes.

**The core inversion:** what matters is "2 tests failed", not "235 of 237 passed." This library suppresses all success noise and delivers only actionable failure information to agents.

---

## 2. What it is

A set of composable Go functions, designed to be called from Mage targets, that make it easy to run a quality gate for Go code. Running a comprehensive quality gate is a non-trivial endeavour; this library provides the building blocks.

Each step is a standalone function that can be invoked multiple times on different data - different scopes, different thresholds, different test suites. Consumers compose them in their own magefiles - the library is not a framework or pipeline engine. It provides building blocks; consumers wire them together.

---

## 3. Separation boundaries

The codebase has five separation boundaries. These are not rigid layers - they are concerns that are kept apart so each can evolve, be tested, and be reused independently.

- **Command execution** - running external tools, capturing output, handling exit codes. Generic and not gate-specific. Any Go program that invokes external commands can use this.
- **File system operations** - reading, writing, and path handling abstracted behind interfaces for testability (mocking in unit tests without touching disk).
- **Artifact handling** - a provenance record of what a build did. Today this is simple (step ID, tool, scope, file path). Architecturally it is aimed at becoming an evidence trail: "I ran a build - what did it do?" This is more than passing filenames around; it is structured metadata that accumulates as a build progresses. Harness temp-directory cleanup failures are returned as errors and use the **same runner-dependent error shaping** as step failures (see §8).
- **Quality gate public API** - the step functions, scopes, option functions, and typed output tokens that consumers import and call.
- **Consumer code** - the consumer's magefiles (or any Go entrypoint), their threshold choices, their lint configuration, their composition decisions. This is their code, not library code.

### Path data realms

- **Canonical logical paths** - forward-slash, `fsnorm`-cleaned, usually root-relative. Used for internal comparisons, artifact names, quality-scope file inventories, exclude matching, and rooted `FileOps` paths.
- **Command cwd** - OS-native path passed to `exec.Cmd.Dir`. Commands run from the gate root unless documented otherwise.
- **Command argv paths** - path format required by the receiving tool. Relative when the tool resolves against command cwd; absolute native when the tool requires host filesystem paths.
- **Host OS paths** - native separator paths for process execution, direct host filesystem adapters, and explicit containment projection.
- **Tool output paths** - tool-emitted paths normalized before comparison or storage. May be absolute, relative, native, or regex-like depending on the tool.

Current path requirements:

- `go test -coverprofile` - root-relative canonical artifact path.
- `go tool cover -func` - same realm as the profile it reads.
- `gremlins -o` - root-relative canonical artifact path.
- Gremlins exclude arguments - regex strings built from canonical root-relative file inventories.
- `go list -f {{.Dir}}` - host package directories; normalize before comparison.
- `gocyclo` inputs - host package directories from `go list`.
- `golangci-lint` config - caller-provided command argv path.
- Custom linter output - artifact path unless the builder requires native.
- Generated artifacts - canonical logical paths through `FileOps`.

---

## 4. Composition model

**Mage's DAG handles runtime ordering.** The library does not impose its own pipeline, orchestrator, or step-sequencing mechanism. Consumers use `mg.Deps` / `mg.SerialDeps` or plain function calls - whatever Mage provides.

**Go's type system handles compile-time safety.** Steps that depend on upstream results require typed output tokens as parameters. You cannot call `Crap` without both a `CoverageOutput` from `Coverage` and a `QualityScopeInventoryOutput` from `QualityScopeInventory`; `Coverage` itself requires a `CoveredTestOutput` from `CoveredTest`. The compiler enforces the dependency shape. It is fine to have separate coverage and CRAP targets, but you must pass the data from A to B; the dependency is structural, not optional.

**Zero-value tokens are rejected at runtime** because Go cannot prevent zero-value struct construction at compile time. See §10 for the full list of accepted runtime-only checks.

**Option functions must not mutate required identity.** When a value type has both required fields (set by the constructor) and optional fields (set by functional options), options receive a private config struct that contains only the optional fields. The constructor validates required inputs, applies options to the config, and assembles the final value. This makes it structurally impossible for an option to overwrite or clear a required field.

**Multiplicity is the consumer's choice.** N coverage checks with different scopes = N function calls with different arguments. The library imposes no "exactly one" constraint on any step.

---

## 5. Scoping

Two scope types serve different purposes:

- **PackageScope** - "what we build and test." The Go package pattern passed to `go test`, `go vet`, `go build`, and similar commands. This is the run target.
- **QualityScope** - "what we consider production code." The measurement boundary for coverage, CRAP, and mutation analysis. Defines a package pattern (measurement seed), optional build tags needed to make that production code visible, exclude segments (path components to filter out of measurement), and test file patterns (files to skip in per-file analysis like gocyclo).

`QualityScopeInventory` is the explicit discovery step for `QualityScope`. It stores package inventory evidence and returns a typed token that downstream measurement steps consume (see "Inventory identity" below). This keeps repeated package discovery out of downstream steps while making the dependency visible to callers.

`CoveredTest` takes both: a `PackageScope` (which packages `go test` runs against) and a `QualityScope` (which packages participate in coverage measurement via `-coverpkg`). When the run target and measurement set should coincide, pass `NewPackageScope(qs.Packages())`.

Quality-scope build tags are part of the metric language, not generic step configuration. They make package and source inventories speak about the same production code across coverage, CRAP, and mutation. Inventory-consuming steps reject build tags passed through generic test or mutation argv; callers put production visibility tags in `QualityScope.Tags()` before producing inventory.

Quality-scope excludes are path-segment exclusions. A segment such as `testdata` excludes matching import paths and matching root-relative source paths from measurement. It is not a glob and is not limited to packages returned by `go list`; file-level tools must translate it to concrete file exclusions where necessary. Test file patterns serve a different purpose: they identify test source files, such as `*_test.go`, to skip inside otherwise in-scope production packages.

### Scope identity and construction

The package pattern is the required identity of `QualityScope`. Build tags, exclude segments, and test file patterns are optional modifiers. The §4 option-function rule applies: options cannot access or mutate the package pattern.

Scope values should store optional data in validated typed form (parsed slices, not comma-joined strings) and render to tool-specific primitives only at command argv or serialization boundaries.

### Inventory identity

`QualityScopeInventoryOutput` is produced once from a specific `QualityScope` and is intended to be passed to multiple downstream steps — `CoveredTest`, `Crap`, mutation scan, and mutation kill — that all use the same scope. That multi-consumer reuse is the normal and intended pattern; the same inventory token is shared across all steps in a single gate run.

The concern is cross-scope misuse: passing inventory produced from `QualityScope A` to a step that receives `QualityScope B`. The token must carry enough scope identity for this to be detectable, and consuming steps must validate that the inventory's scope matches the `QualityScope` they receive.

### Scope-to-command translation

A `QualityScopeInventoryOutput` and its matching `QualityScope` describe one measurement boundary, and every measurement step uses that same boundary consistently.

Command inputs stay minimal and predictable: package flags get import-path CSV, whole-subtree excludes become directory regex, and file regex is used only when broader matching would remove production code that should stay in scope. Test-file patterns collapse to one package-directory regex per in-scope package, rather than one regex per file.

Scope modifiers are applied where they belong:

- **Inventory discovery** — package seed and build tags only; excludes and test-file patterns do not narrow discovery.
- **Coverage instrumentation** — `-coverpkg` reflects package/path excludes; test-file patterns do not change it.
- **Coverage and CRAP summaries** — use a filtered profile when excludes or test-file patterns are active; otherwise use the profile from `CoveredTest`.
- **CRAP complexity** — one gocyclo pass over in-scope package directories; file-level skips apply when reading gocyclo output.
- **Mutation scan and kill** — same scope-derived command inputs; scan adds dry-run. If scope excludes every package, mutation fails before gremlins starts.

`Duration` is outside this translation; see **Duration scope alignment** below.

### Duration scope alignment

Duration filtering (exclude segments, test file patterns) must not drift from the producing test/covered-test scope. When `Duration` consumes a test output produced by `CoveredTest`, the filtering scope should derive from that covered-test data rather than requiring the caller to independently supply a matching `QualityScope`.

---

## 6. What the library requires of consumers

- **Every threshold is consumer-specified.** No defaults. Calling a threshold-bearing step without its threshold option is a validation error, not silent acceptance of a built-in value.
- **Thresholds have physical meaning - zero is not universally valid.** Whether zero is accepted depends on what the metric measures. The library validates what is **feasible** for each metric: values outside the meaningful range (e.g. negative percentages or scores) are rejected just like impossible zeros - not because of a blanket rule about zero, but because they are not interpretable as real thresholds:

  - Coverage (and Mutation Coverage) 0% means "I don't require coverage" - a valid lower bound that instantly passes and effectively disables the enforcement
  - CRAP 0 is physically impossible (every function has complexity >= 1, so CRAP >= 1) - always a validation error
  - MutationSites 0 means "no file may have any mutable code" - practically impossible, always a validation error
  - Duration 0 means "no test may take any time" - impossible, always a validation error
  - Mutation Kill Rate 0 means "I don't need mutations to be killed" - a valid lower bound that instantly passes and effectively disables the enforcement

- Tool versions are consumer-specified via tool spec strings (e.g. `"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"`)
- Required inputs use typed parameters and constructors (see §4). Optional behavior is generic tool argv (`*Args` constructors) by default; typed option functions exist only when the library must enforce an invariant (for example, build tags as scope modifiers rather than generic argv)
- Constructors that accept required interface dependencies must reject nil immediately, not defer validation to later method calls
- `magefiles/` is this repo's own dogfooding. It uses the same public API as any downstream consumer. It is the only place this project expresses opinions about threshold values, lint configuration, or tool versions. The `gate` package itself has no configuration-format dependency.

---

## 7. Tool pinning

External tools (golangci-lint, deadcode, gremlins, gocyclo) are always version-pinned via consumer-provided tool specs (see §6). The library has no defaults for tool versions.

If the pinned version already exists locally, use it directly - avoid redundant downloads. If not available locally, fall back to `go run package@version`. A local binary whose **version cannot be probed** yields an **error** (there is no silent fallback to `go run` in that case). Go toolchain commands (`go vet`, `go build`, `go test`, `go tool cover`) use the ambient Go installation and are not subject to tool pinning.

---

## 8. Output model

**Package split:** `cmdrunner` is execution-only (run commands, capture output, resolve tools). `OutputMode` and subprocess display filtering live in `gate` - they exist to support agent-first presentation vs full human logs.

- **Agent mode:** no success subprocess output on the display writers when using the supported wrapper with `OutputModeAgent`. Failures emit curated diagnostics to `stderr`, telling the agent precisely what went wrong, what to do to fix it, and enough context to act - without satisficing success messages. All subprocess success noise (pass tallies, timing summaries, celebration) is suppressed on the display side for that wiring.
- **Verbose mode:** full subprocess output on the display writers for human review; step failures return raw tool/sentinel errors (no `DiagnosticError` wrapper).
- **`OutputModeAgent` vs `OutputModeVerbose`** are the two supported values on that wrapper - not inferred from environment variables alone
- **Output mode is a property of the runner**, not individual steps. Steps are output-mode-unaware.
- **Consumer owns display output**, and the gate emits only minimal progress lines as proof-of-life signals so agents don't interpret total silence on long operations as hung.

---

## 9. Mutation strategies

**Dry-run path (quality-scope-only):** `QualityScopeInventory` performs package discovery once, then `MutationRunner.Scan` performs one gremlins dry-run from that inventory. **QualityScope** (§5) defines the measurement boundary for that run. The scan writes an artifact; separate step functions enforce thresholds against that artifact without re-executing gremlins dry-run.

**Two checks, one dry-run:** `MutationSites` enforces a per-file site budget. `MutationCoverage` enforces a covered-mutant share. Both are optional in policy terms (`MutationCoverage` is only applied when a consumer supplies a minimum), but a typical pre-commit gate at least enforces a site budget. The intended composition is **one** `Scan` call, then `MutationSites` and, when configured, `MutationCoverage` on the same scan output.

**Full-run path:** `MutationKills` (or `Kill` + `MutationKillRate`) runs gremlins without dry-run to measure **kill rate**. It is on-demand, not the same cost model as the dry-run checks, and is not implied by the dry-run path. The library also exposes **kills-based adapters** to evaluate site budget or mutation coverage from full-run output when a caller explicitly chooses that flow.

The stable contract for callers remains **named metrics** (per-file site load, dry-run coverage share, kill rate), not a particular JSON layout. This document does not name a vendor tool or on-disk format - callers should depend on those metrics, not on a particular engine. **Current releases pin to a single mutation engine**; module paths and CLI details live in the README and package godoc. **Neutrality** here means keeping those metrics stable if the implementation changes, not offering a plug-in mutation engine in the public API today.

---

## 10. Key invariants

- **`-count=1` is always injected into `go test` invocations.** Without it, Go's test cache does not replay coverage instrumentation data - coverage literally does not work. This is a correctness requirement, not a policy preference.
- **Duration thresholds apply to per-test completions, not package wall-clock.** The Duration step checks each `go test -json` test completion event's `Elapsed` value. When Go emits both a parent test and subtests, both are checked independently; package-level completion elapsed time, coverage instrumentation, and test binary build time do not count.
- **Agent display mode emits only errors and proof-of-life tokens** (see §8 for the full output model).
- **No step runs without an explicit threshold** when one applies. Missing thresholds are validation errors.
- **The consumer surface is typed parameters and constructors plus option functions**, not string-based step IDs or opaque configuration keys. Typed composition is primary.
- **Runtime validation is the exception, not the default.** The following categories are accepted as runtime-only checks because Go cannot express them at compile time: zero-value output token rejection, artifact store/root provenance matching, and external process input validation. All other required-input enforcement should prefer construction-time or compile-time invariants.

---

## 11. Verification

How we know the design is satisfied:

| Requirement | Evidence |
|-------------|----------|
| Consumer composes via function calls | Import `gate`; call standalone functions with explicit dependencies |
| Typed tokens enforce prerequisites | CRAP requires `CoverageOutput`; zero-value tokens rejected at runtime |
| No config-format dependency | `gate` package has no TOML/YAML/config import |
| Artifact cleanup | Default uses OS temp; library-owned dirs cleaned up on step completion |
| Dogfood parity | `magefiles/magefile.go` uses the same public API as any consumer |
| No success output beyond proof-of-life | Agent-mode scenarios in `features/*.feature` verify no output on success where those scenarios exist |
| Format step contract | `features/format.feature` verifies apply (`fmt`) dispatch and output |
| Explicit thresholds | Calling a threshold step without its option is a validation error |
| Options cannot mutate required identity | Option functions receive private config excluding required fields |
| Scope values store typed data | Excludes, tags, patterns stored as parsed slices, not CSV strings |
| Nil deps rejected at construction | Constructors accepting required interfaces fail fast, not at method time |
| Runtime checks enumerated | Only zero-value tokens, provenance, and external input defer to runtime |
