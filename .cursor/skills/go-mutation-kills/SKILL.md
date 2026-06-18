---
name: go-mutation-kills
description: >-
  Run full gremlins mutation testing (mage mutationKills), read the printed summary
  (kill rate, counts, survivors by file), or persist JSON for extra analysis.
  Use for mutation runs, kill rate, LIVED survivors, gremlins reports, or
  "which tests need expansion from mutations".
---

# Go Mutation Kills

- **`mage mutationKills`** prints a multi-line human summary; you do not need a JSON file to see which files had survivors
- Expect several minutes on a typical workstation. Temp artefacts are cleaned up; no JSON is left on disk

```bash
go run github.com/magefile/mage@v1.17.0 mutationKills
```

## Interpreting results

| Status | Meaning |
|--------|---------|
| `KILLED` | Test suite detected the mutant — good. |
| `LIVED` | No test caught the mutation — **missing or weak assertion**. Primary expansion target. |
| `NOT COVERED` | Code not in `coverpkg` / excluded paths - signals missing tests |
| `TIMED OUT` | Mutant caused very slow or non-terminating behaviour; may need timeout/cancellation tests or Gremlins timeout tuning |

## gate.toml reference (key fields)

```
[gremlins]        tool_spec = "module/path/cmd/gremlins@version"   # shared pin for sites + kills
[quality_scope]   exclude = [...]   # segments subtracted from coverpkg (shared boundary for mutation + coverage)
[thresholds]      mutation_kills_min_rate = N   # omit → check disabled
```

## Distinction from MutationSites

| | MutationSites | MutationKills |
|-|---------------|---------------|
| Mode | `--dry-run` (no tests per mutant) | Full run (tests execute per mutant) |
| Gate path | Part of `mage gate` precommit | On-demand (`mage mutationKills`) |
| Cost | Seconds | Minutes |
| Output | Site count per file | Headline counts + optional survivors-by-file table |
