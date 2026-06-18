// Vision: go doc anchor; the package block below is the consumer-facing contract (tokens, output modes, scopes).

// Package gate provides an agent-first quality gate library for Go projects.
//
// The library extracts actionable signal from build tool output and formats
// failures as structured prompts (ERROR/Fix/Hint). Success produces zero
// diagnostic noise; failure tells the agent exactly what to do next.
//
// # Typed Composition
//
// Steps are standalone functions that return opaque tokens enforcing
// producer-consumer ordering at compile time:
//
//   - [Test] (package-only scope) produces [TestOutput]
//   - [CoveredTest] produces [CoveredTestOutput]; [CoveredTestOutput.TestRun] yields [TestOutput]
//     for [Duration] (check its error)
//   - [Coverage] requires [CoveredTestOutput] and produces [CoverageOutput]
//   - [Crap] requires [CoverageOutput]
//   - [Duration] requires [TestOutput]
//   - Independent checks (no token chain): [Lint], [Format], [Compile], [Vet], [Deadcode], and [Test] each take a
//     [PackageScope] run target; [MarkdownLint] runs gomarklint from repo root with consumer argv only.
//     [MutationRunner.Scan] runs gremlins --dry-run and returns [MutationScanOutput]
//     (correlation + [QualityScope] only; the [ArtifactStore] passed to [NewMutationRunner] holds mutations.json).
//     [NewMutationRunner] holds resolver/store/fileOps/runner deps and exposes [MutationRunner.Scan] and
//     [MutationRunner.Kill] (full run metrics only; compose with [MutationKillRate] for thresholds).
//   - [MutationSites] and [MutationCoverage] enforce metrics from the stored scan artifact using the
//     [MutationScanOutput] token returned from [MutationRunner.Scan] (the token binds the store for reads).
//     [MutationKills] is a separate on-demand full run that returns [MutationKillsOutput] for kill rate.
//     [MutationCoverageFromKills] and [MutationSitesFromKills] reuse full-run parsed metrics for the
//     coverage percentage and per-file site budget, respectively, without a second gremlins run or store read.
//
// Zero-value tokens are rejected at runtime because Go cannot prevent their
// construction at compile time.
//
// # Quality vs artifacts
//
// The gate verifies quality; consumer release targets produce artifacts.
// [PackageScope] names run targets: which packages [Lint], [Format], [Compile], [Vet], [Deadcode],
// and [Test] execute against, and which packages [CoveredTest] runs [go test] over.
// [QualityScope] is the measurement boundary only: coverpkg seed for [CoveredTest] and
// [MutationRunner.Scan] / [MutationKills], plus [Exclude] and [TestFilePatterns] for coverage, CRAP, and mutation
// thresholds. It does not name run targets for lint, vet, compile, or deadcode.
//
// A single merged [go test] run with -coverprofile but without a scoped -coverpkg list can still
// count every package in the module, so fixtures or BDD harness code may appear as 0% blocks and
// depress a raw aggregate. The value enforced by [Coverage] is the [go tool cover] total on the
// quality-scoped profile: [CoveredTest] seeds -coverpkg from [QualityScope] with [Exclude] segments
// removed from the list, and [Coverage] may filter the profile again for those same segments.
//
// # Mutation: dry-run vs full-run
//
// [MutationRunner.Scan] (gremlins --dry-run) is scoped by [QualityScope] only for mutation inputs.
// [MutationSites] and [MutationCoverage] are separate checks over the stored scan artifact; a single
// [Scan] can feed both without a second dry-run. [MutationRunner.Kill] and [MutationKillRate] (or
// the bundled [MutationKills] entrypoint) implement the full-run kill-rate path. [MutationSitesFromKills]
// and [MutationCoverageFromKills] are explicit adapters when kill-run output should satisfy dry-run-style checks.
// [Compile] runs go build to confirm a [PackageScope] compiles; it is not a release or
// packaging API. [CompileArgs] forwards flags to the go tool unchanged—prefer compile-behavior
// flags only, and use a separate consumer-owned target (for example go build -o, GoReleaser)
// for binaries and cross-platform matrices.
//
// Optional command argv is exposed through one unified family:
// [CompileArgs], [VetArgs], [TestArgs], [DeadcodeArgs], [MarkdownLintArgs], [LintArgs], [CrapArgs], and [MutationArgs].
// Each appends consumer tokens after the step's fixed command prefix.
//
// # Dependencies
//
// Every step receives its dependencies as explicit parameters: a
// [CommandRunner] for subprocess execution, [FileOps] for filesystem
// access, and an [*ArtifactStore] for steps that produce or consume
// inter-step artifacts. The call site is self-documenting.
//
// # Output Modes
//
// [NewDisplayRunner] selects [OutputModeAgent] or [OutputModeVerbose].
// [RunnerOutputMode] consults optional [OutputModeProvider].
//
// # Errors
//
// Pre-run configuration errors use [ValidationError] and sentinel errors
// ([ErrInvalidOption], [ErrNilDependency], [ErrPackageScopeEmpty], [ErrQualityScopeEmpty]).
// Post-run tool failures use [DiagnosticError] with ERROR/Fix/Hint fields
// in agent display mode, or raw sentinels in verbose display mode. See [errors.go] for
// the full taxonomy.
//
// # Configuration
//
// The library has no opinion on configuration format. Thresholds and
// options are Go function calls: [MinPercent], [MaxScore], [MaxSeconds],
// [MaxSites], [MinMutationCoverage], [LintConfig], [NewLintToolchain], etc.
// Compose [MutationRunner.Scan] (dry-run) with
// [MutationSites] and optionally [MutationCoverage] using the returned scan token.
// How consumers obtain values (TOML,
// YAML, flags, environment variables) is outside the library's scope.
package gate
