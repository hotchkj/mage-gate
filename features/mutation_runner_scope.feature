Feature: Mutation runner scope translation
  The mutation runner translates quality scope into concrete gremlins command input.

  Background:
    Given the package scope is "./..."
    And the quality scope is "./..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the module "example.com/mod" has package "example.com/mod/internal/testutil" at "/mod/internal/testutil"
    And the module "example.com/mod" has package "example.com/mod/vendor/lib" at "/mod/vendor/lib"
    And the tool spec for "mutationscan" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
    And the tool spec for "mutationkills" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

  Scenario: scan includes quality-scope package seed
    Given the quality scope is "./internal/..."
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil
      --dry-run
      """

  Scenario: scan narrows the measurement boundary while translating mutation excludes
    Given the quality scope excludes "testutil"
    And the quality scope excludes "vendor"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app
      --exclude-files=^internal/testutil(/|$)
      --exclude-files=^vendor/lib(/|$)
      --dry-run
      """

  Scenario: overlapping excludes translate deterministically for mutation suppression
    Given the quality scope excludes "internal"
    And the quality scope excludes "internal/testutil"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/vendor/lib
      --exclude-files=^internal/app(/|$)
      --exclude-files=^internal/testutil(/|$)
      --dry-run
      """

  Scenario: full mutation run uses the same narrowed measurement boundary
    Given the quality scope excludes "testutil"
    And a mutation kills min rate of 0 percent
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/vendor/lib
      --exclude-files=^internal/testutil(/|$)
      """

  Scenario: test file patterns collapse to package-level exclude regexes
    Given the fixture contains test file "internal/app/foo_test.go"
    And the fixture contains test file "internal/app/bar_test.go"
    And the fixture contains test file "vendor/lib/lib_test.go"
    And the quality scope test file patterns include "*_test.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --exclude-files=^internal/app/.*_test\.go$
      --exclude-files=^vendor/lib/.*_test\.go$
      --dry-run
      """

  Scenario: excluded packages do not redundantly emit test-file pattern regexes
    Given the fixture contains test file "internal/app/app_test.go"
    And the fixture contains test file "internal/testutil/util_test.go"
    And the fixture contains test file "vendor/lib/lib_test.go"
    And the quality scope excludes "testutil"
    And the quality scope test file patterns include "*_test.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/vendor/lib
      --exclude-files=^internal/app/.*_test\.go$
      --exclude-files=^internal/testutil(/|$)
      --exclude-files=^vendor/lib/.*_test\.go$
      --dry-run
      """

  Scenario: explicit gremlins exclude flags are asserted literally when the implementation uses them
    Given the quality scope excludes "testutil"
    And the quality scope excludes "vendor"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app
      --exclude-files=^internal/testutil(/|$)
      --exclude-files=^vendor/lib(/|$)
      --dry-run
      """

  Scenario: source files outside package inventory collapse to segment directory regex
    Given the fixture contains source file outside the package inventory "magefiles/magefile.go"
    And the fixture contains source file outside the package inventory "testdata/failures/calc.go"
    And the quality scope excludes "testdata"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --exclude-files=^testdata(/|$)
      --dry-run
      """

  Scenario: multiple unlisted files under one segment emit one directory regex
    Given the fixture contains source file outside the package inventory "testdata/failures/calc.go"
    And the fixture contains source file outside the package inventory "testdata/failures/other.go"
    And the fixture contains source file outside the package inventory "testdata/deep/nested/x.go"
    And the quality scope excludes "testdata"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --exclude-files=^testdata(/|$)
      --dry-run
      """

  Scenario: file-specific exclude emits concrete regex when directory regex would be too broad
    Given the fixture contains source file outside the package inventory "internal/app/helper.go"
    And the quality scope excludes "internal/app/helper.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --exclude-files=^internal/app/helper\.go$
      --dry-run
      """

  Scenario: quality-scope tags feed inventory and mutation scan
    Given the quality scope has build tag "mage"
    And the quality scope has build tag "integration"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      list
      -e
      -tags=mage,integration
      -f
      <package-inventory-format>
      ./...
      """
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --tags=mage,integration
      --dry-run
      """

  Scenario: mutation scan rejects generic build-tag arguments
    Given mutation has an extra argument of "--tags=consumer" specified
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the step fails
    And the error is ErrInvalidOption
    And the gremlins dry-run command did not run

  Scenario: root module files are escaped as anchored file regexes
    Given the module "example.com/mod" has package "example.com/mod" at "/mod"
    And the fixture contains source file "main.go"
    And the quality scope excludes "main.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod,example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --exclude-files=^main\.go$
      --dry-run
      """

  Scenario: metacharacter-bearing source paths are quoted before regex emission
    Given the module "example.com/mod" has package "example.com/mod/weird(a)/pkg" at "/mod/weird(a)/pkg"
    And the fixture contains source file "weird(a)/pkg/f+.go"
    And the quality scope excludes "weird(a)/pkg/f+.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib,example.com/mod/weird(a)/pkg
      --exclude-files=^weird\(a\)/pkg/f\+\.go$
      --dry-run
      """

  Scenario: slash variants normalize before exact regex emission
    Given the module "example.com/mod" has package "example.com/mod/slashy/pkg" at "/mod/slashy/pkg"
    And the fixture contains source file "slashy/pkg/x.go"
    And the quality scope excludes "slashy\pkg\x.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/slashy/pkg,example.com/mod/vendor/lib
      --exclude-files=^slashy/pkg/x\.go$
      --dry-run
      """

  Scenario: broad test file patterns fail closed when every mutation candidate is excluded
    Given the quality scope test file patterns include "*.go"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the step fails
    And the error is ErrAllPackagesExcluded

  Scenario: all packages excluded fails before gremlins runs
    Given the quality scope excludes "internal"
    And the quality scope excludes "vendor"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the step fails
    And the error is ErrAllPackagesExcluded
    And the gremlins dry-run command did not run
