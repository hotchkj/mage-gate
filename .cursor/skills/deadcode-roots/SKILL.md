---
name: deadcode-roots
description: >-
  Maintain compile-only deadcode roots for gate.
  Keeps deadcode reachability analysis stable as production constructors evolve.
---

# Deadcode Roots

Use this when updating production-only constructors in `gate` that are banned from tests by lint.

## What This Is For

Deadcode roots anchor symbols that are:
1. Legitimately exported public API in `gate`
2. Banned from tests by `forbidigo` / `depguard` lint rules
3. Therefore unreachable from the test call graph by design

The only symbols that belong here are production-only constructors. Everything else in `gate` should be reachable from tests. If deadcode reports a symbol not in this list, that signals missing test coverage, not a missing root.

**Do not add roots for:**
- Option constructors (`MinPercent`, `MinKillRate`, etc.) — tests call these directly
- Step functions (`Lint`, `Test`, etc.) — tests call these directly
- Data types, error types, tokens — tests use these directly
- Internal packages — `internal/` packages are not public API and must not have roots files

## Update Steps

1. Keep tag parity:
   - `gate/deadcode_roots_build_tag.go` constant
   - `//go:build ...` in `gate/deadcode_roots.go`
2. Add/remove anchors in `deadcode_roots.go:init()` only for production-only constructors
   that lint bans from tests.
3. Keep anchors side-effect free (compile-only references, no runtime behavior).

## Quick Validation

```bash
go test ./gate -tags qgdcroots_f4c8e91b3d7042a6b5c1e8f9037d2a61 -run '^$' -count=1
go run golang.org/x/tools/cmd/deadcode@v0.31.0 -test -tags qgdcroots_f4c8e91b3d7042a6b5c1e8f9037d2a61 ./gate
```

Note: `-test` is explicitly required. The `gate` library no longer injects it automatically;
callers control reachability mode via `DeadcodeArgs`.

Do not assume success without **exit code 0** and output that matches expectations; if you ran the commands, investigate missing or surprising logs before continuing.
