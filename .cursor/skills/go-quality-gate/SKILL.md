---
name: go-mage-gate
description: >-
  Run the full Go quality gate before merge or after risky changes.
  Use this skill to execute the gate and rely on script output for failures.
---

# Go Quality Gate

Use this for full project validation. The gate itself reports which checks failed and why.

```bash
go run github.com/magefile/mage@v1.17.0 gate
```

## Common Modes

Individual stages (lint, deadcode, vet, compile, test, mutationsites, mutationcoverage, coverage, crap, duration):

```bash
go run github.com/magefile/mage@v1.17.0 lint
```

**On-demand mutation kill rate check** (not part of the main gate):

```bash
go run github.com/magefile/mage@v1.17.0 mutationkills
```

The `mutationsites` stage (precommit-friendly) counts insertion points per file; `mutationkills` (optional, merge-gate) measures actual kill rate against a threshold.

## Agent verification

Do not assume a command succeeded without **exit code 0** and output that matches expectations (e.g., tests ran, gate finished cleanly). If you ran it, you own the result—investigate missing or surprising logs before continuing.

## Notes

- Requires Go installed and available on `PATH`.
- Mage auto-discovers `magefiles/` at repo root; no flags needed.
- For quick, package-level feedback during development, use `go-fast-validation`.
