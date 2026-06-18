---
name: go-afero-memmapfs-paths
description: Avoid test failures from afero.MemMapFs absolute-path pollution. Use when working with afero.MemMapFs, MkdirTemp, Walk, filepath.Abs, or ResolveWithinRoot in Go tests. Critical for cross-platform path handling in memory-backed filesystem tests.
disable-model-invocation: true
---

# afero MemMapFs Path Handling

Authoritative detail: `docs/kb/go.md` Path Handling.

## Principle

Production code has one path contract: canonicalize inputs, keep internal path data canonical, and convert to OS-native only at explicit seams (`ResolveWithinRoot`, exec, disk open). Test doubles must produce inputs that satisfy that contract. Do not add production branches that guess whether code is running under tests.

## The Footguns

### 1. TempDir with empty base

`afero.TempDir(fs, "", pattern)` uses `os.TempDir()` to choose the base path string. With `afero.MemMapFs`, that inserts real OS temp prefixes such as `/tmp/...` into the in-memory key space. Those keys are not under the same lexical tree as paths from `Walk(".")`.

### 2. MkdirAll with absolute paths corrupts Walk

Any `MkdirAll` with an absolute path (e.g. `/Users/runner/work/...`) in MemMapFs creates children of root `/` whose `FileData.Name()` includes the leading `/`. When `Walk(".")` later reads root children via `Readdirnames`, `filepath.Split` strips the leading `/`, producing a base name like `"Users"`. Walk reconstructs `filepath.Join(".", "Users")` = `"Users"`, but MemMapFs stores the entry under key `"/Users"`. The lookup fails with `ErrNotExist` and Walk **aborts before visiting any sibling that sorts after the phantom entry**.

This is the root cause of the `mutation_runner_scope.feature:108` CI failure: `ensureMutationArtifactDir` called `ResolveWithinRoot` (which uses `filepath.Abs`) to resolve a relative temp dir to an absolute OS path, then `MkdirAll`'d that absolute path in MemMapFs. On macOS CI the resulting `/Users/…` entry broke Walk before it reached fixture `.go` files. On Windows the absolute path has a drive-letter volume (`D:\...`) which is separate from the MemMapFs root `\`, so the bug was hidden.

Proof: `gatetest/doubles_test.go` `TestGatetest_MemoryFileOps_AbsoluteMkdirAllCorruptsWalk`.

## Rules

1. **MemMapFs injected keys:** never use host-rooted strings produced by `filepath.Abs` or `ResolveWithinRoot` as MemMapFs path keys (`MkdirAll`, `MkdirTemp` base relative to Fs root, opens, Walk roots). Doing so corrupts Walk and separates keys from lexical trees rooted at `"."` / `\`. Prefer the canonical logical artifact path (`artifactPaths`-style projections, root-relative forward slashes) already used for containment validation. Practical corollary: do not pass `ResolveWithinRoot` / `filepath.Abs` outputs into MemMapFs-backed `MkdirAll` or opens — use the harness’s original relative canonical path (for example `sr.artifactSubdir`) for those calls. If a test must prove the raw `afero.MemMapFs` corruption mode, use `afero.NewMemMapFs` directly rather than routing the pollution through `MemoryFileOps`.
2. **Containment without poisoning MemMapFs:** prefer the harness `artifactPaths` projections and rooted `FileOps` containment for artifact directories. Use `ResolveWithinRoot` only for explicit host OS projections such as native command destinations or direct host adapters; never use its returned string as a MemMapFs key.
3. `MemoryFileOps` must canonicalize every path argument it receives, including `MkdirTemp`'s `dir`.
4. In `MemoryFileOps.MkdirTemp`, treat empty `dir` as `"."` before calling `afero.TempDir`, then canonicalize the returned path.
5. Test artifact directories should be relative unless the test is explicitly checking absolute path behavior.
6. Fakes that write tool output should use the same resolver the harness uses to read the output back, not bespoke `filepath.Abs` or lexical join logic.
7. Never fix MemMapFs inconsistencies by adding production root-guessing or test-vs-production branches.

## Checks

Before changing path behavior, search for:

- `MkdirTemp`
- `afero.TempDir`
- `filepath.Abs`
- `ResolveWithinRoot`
- `MkdirAll` (verify arguments are relative when the FileOps could be MemMapFs)
- `gatetest.Gremlins`

## References

- https://github.com/hotchkj/mage-gate/issues/8 — containment vs MemMapFs pattern and merge/CI pitfalls
- `docs/kb/go.md`: canonical path contract and afero TempDir warning
- `docs/kb/coding-standards.md`: path canonicalization requirements
- `internal/fsnorm/fsnorm.go`: canonical lexical path helpers
- `gatetest/fileops.go`: memory filesystem fake
- `gatetest/doubles_test.go`: proof test for MkdirAll absolute-path corruption
