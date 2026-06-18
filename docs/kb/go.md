# Go

## Error Handling

- Prefer compiler failures over runtime failures; runtime checks only where inherently runtime
- Return errors, never print them (`log.Fatal`, `fmt.Println`, `os.Exit` are not error handling)
- Declare sentinels as `var ErrXxx = errors.New("...")`; wrap with `%w`; classify with `errors.Is`/`errors.As`
- Custom error types carry structured context and `Unwrap()` to the sentinel

## API Design

- Design one entrypoint with an options struct; method variants become fields
- Use typed parameters over flag combinations
- Avoid creating N function variants for different parameter combinations
- Avoid compatibility shims for internal API changes

## Construction and Option Patterns

- Functional options that configure a value type containing required fields must mutate a private config struct that excludes those fields, not the value type itself
- Constructors validate required inputs and build the final immutable value from validated parameters plus the private option config; options never touch the final object
- Prefer opaque validated value types or sealed constructors for scopes, thresholds, tool specs, and other boundary values — a zero value that compiles and fails only at runtime is an acceptable Go limitation, but document that compromise at the type
- Store validated domain data in typed form (slices, domain types) internally; render to primitives (CSV strings, raw ints, argv tokens) only at output boundaries
- When a constructor returns `(T, error)`, it must reject all detectable invalid states immediately — do not defer validation to method calls if the constructor has the information

## Dependencies

- Inject dependencies for testability; design mockable seams
- Pass dependencies explicitly via constructors or option functions
- Make required dependencies required parameters so missing arguments fail to compile
- Constructors that accept interface dependencies must fail fast for nil required interfaces; returning a valid-looking object that fails later on method calls hides the programming error
- Optional dependencies (genuinely optional — correctness does not depend on their presence) may default if nil but document them as optional
- Never silently default required dependencies — this hides programming errors that should fail at construction

## Production Wiring

- Production implementations (`NewProductionRunner`, `NewProductionFileOps`) do real IO behind injected interfaces — they are untestable in unit/feature tests by design
- Exclude production wiring from unit and feature coverage; add to linter exclusions or deadcode roots as needed
- If code cannot be tested without violating test isolation, it belongs behind an interface with a fake — not in a test with a `//nolint` comment

## Testing

- Use channels and WaitGroups for synchronization; never `time.Sleep`
- Skip tests with `t.Skip` and a stated reason when IO is unavoidable; restore before committing
- Prefer fakes or `afero` in-memory mocks for filesystem testing
- Keep `net`, `net/http`, `crypto/tls`, `os/exec`, and `syscall` imports out of unit tests
- Use `httptest` and real listeners only in integration tests with documented justification
- Assert with `errors.Is`/`errors.As` against stable package-level sentinels
- Do not match `err.Error()` strings or compare formatted message text unless the contract under test is explicitly the human-readable wording

## Path Handling

- After `fsnorm.Canonical`, use `fsnorm.Rel` for relative paths and `path.Dir`, `path.Join`, `path.Base`, or `fsnorm` wrappers for other canonical path manipulation; never use `filepath.Rel`, `filepath.Dir`, or `filepath.Join` on canonical strings
- `filepath.*` is OS-native: it can reintroduce platform-specific separators and `Rel`/`Join` behavior. Use it only before canonicalization or at explicit OS-native seams
- Legitimate `filepath.*` use on canonical data is limited to classification (`filepath.IsAbs`) and security containment boundaries (`filepath.Abs` plus `filepath.Rel` for escape detection, e.g. `ResolveWithinRoot`)
- `afero.TempDir(fs, "", pattern)` uses `os.TempDir()` to choose the base path string. In `afero.MemMapFs`, that can create keys like `/tmp/...` which are outside the same lexical tree as `Walk(".")`; memory fakes must normalize empty temp dirs to `"."` and canonicalize the returned path instead of changing production path logic

## Tooling

- Pin module versions on all `go run` invocations (hooks, Taskfile); avoid `@latest`

## Version-Sensitive Review

- Check the repo's effective Go version before flagging patterns whose correctness changed across releases
- When language semantics, standard-library behavior, or common concurrency advice changed across Go releases, cite repo-local version evidence before calling something a bug
- Prefer repo-local lint/toolchain evidence over stale generalized Go advice
- If a review claim conflicts with the repo's Go version or lint configuration, stop and reconcile before editing

## Conventions

- Follow existing codebase patterns; read surrounding code before writing
