---
name: gofumpt-ci-parity
description: >-
  Match golangci-lint gofumpt (extra-rules) when formatting locally. Prevents
  “gofumpt -w passed, CI lint failed” drift.
---

# Gofumpt CI parity

`.golangci.yml` enables gofumpt with `extra-rules: true`. The CLI default is without extra rules.

Always format with:

```bash
go run mvdan.cc/gofumpt@v0.9.2 -extra -w .
```

Plain `gofumpt -w` (no `-extra`) is not equivalent and can leave files that still fail lint.

Do not assume success without exit code 0; re-run lint or gate if unsure.
