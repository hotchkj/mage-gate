---
name: go-fast-validation
description: >-
  Minimal targeted testing for Go changes. Quick validation without full gate.
  Use for small localized changes, targeted test runs, and pre-commit format checks.
  Emphasizes gofmt over manual edits and targeted lint/test when possible.
---

# Go Fast Validation

Quick validation for single file/package changes during development.
For full merge assurance, see `go-mage-gate`.

## Agent verification

Do not assume `go`, `mage`, or lint tools succeeded without **exit code 0** and output that matches expectations. If you ran the command, you own the result—investigate missing or surprising logs before continuing.

## When to Use

- Single file or package modification  
- Rapid feedback during active development  
- Pre-commit checks on changed files

## Validation Steps

### 1. Format (Always `gofmt`, Never Manual Edits)

```bash
gofmt -w <file.go>                    # Single file
go fmt ./path/to/pkg                  # Package
git diff --name-only HEAD | grep '\.go$' | xargs gofmt -w  # Changed files
```

### 2. Compile Package

```bash
go build ./path/to/pkg
```

### 3. Targeted Lint & Deadcode

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1 run ./path/to/pkg
go run golang.org/x/tools/cmd/deadcode@v0.31.0 -test ./path/to/pkg
```

Note: `-test` is a consumer choice, not injected by the library. For library repos with no
`main` packages, `-test` is required for deadcode to have reachability entry points. Omit
it only when analyzing a program that has real `main` packages.

### 4. Targeted Tests

```bash
go test ./path/to/pkg -v -count=1
```

## File-Level Validation

Go validates packages, not files. Derive package from file path (e.g., `file.go` in `pkg/` → `./pkg`),
then validate: `go build ./pkg`.

## Failure Diagnosis

**Lint/deadcode:** Read error, fix source, re-run.  
**Tests fail:** Use `-v -run TestName` to isolate.  
**Build fail:** Fix compiler error.
