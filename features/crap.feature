Feature: CRAP step
  The CRAP step checks that no function exceeds the configured
  complexity-to-coverage ratio.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a CRAP threshold of 8
    And a coverage threshold of 0
    And the module "example.com/mod" has package "pkg" at "/mod/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
    And the tool spec for "crap" is "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

  Scenario Outline: passes when below threshold
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And <output>

    Examples:
      | mode   | output                                     |
      | agent  | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when above threshold
    Given function "Validate" has cyclomatic complexity 15
    And the codebase has 50% test coverage
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step fails
    And <output>

    Examples:
      | mode   | output                                     |
      | agent  | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: tool resolution for gocyclo
    Given function "Bar" has cyclomatic complexity 5
    And the codebase has 75% test coverage
    And the local tool for "crap" is "<state>"
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then <outcome>
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./zz_bdd_not_default/...
      """
    And the command `<command>` is run with arguments:
      """
      <arguments>
      """

    Examples:
      | state       | outcome                   | command | arguments                                                                                         |
      | matching    | the tool runs locally     | gocyclo | -over 0 zz_bdd_not_default/pkg zz_bdd_not_default/testutil                                        |
      | mismatched  | the tool runs via go run  | go      | run github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0 -over 0 zz_bdd_not_default/pkg zz_bdd_not_default/testutil |
      | missing     | the tool runs via go run  | go      | run github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0 -over 0 zz_bdd_not_default/pkg zz_bdd_not_default/testutil |
      | unprobeable | the tool is not available | none    |                                                                                                   |

  Scenario: passes crap extra args through
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the output mode is agent
    And crap has an extra argument of "arg1" specified
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
      -over
      0
      arg1
      zz_bdd_not_default/pkg
      zz_bdd_not_default/testutil
      """

  Scenario: uses the configured quality scope for measurement
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./internal/...
      """
    And the command `go` is run with arguments:
      """
      run
      github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
      -over
      0
      internal/app
      """

  Scenario: quality scope excludes narrow the measurement boundary
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And the command `go` is run with arguments:
      """
      test
      ./zz_bdd_not_default/...
      -json
      -coverprofile=<artifact>/coverage.out
      -coverpkg=example.com/mod/zz_bdd_not_default/pkg
      -count=1
      """
    And the command `go` is run with arguments:
      """
      tool
      cover
      -func=<artifact>/coverage-filtered.out
      """
    And the command `go` is run with arguments:
      """
      run
      github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
      -over
      0
      zz_bdd_not_default/pkg
      """

  Scenario: CRAP runs one gocyclo command for all scoped package directories
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
      -over
      0
      zz_bdd_not_default/pkg
      zz_bdd_not_default/testutil
      """

  Scenario: test file patterns filter CRAP coverage input
    Given function "Validate" has cyclomatic complexity 5
    And the codebase has 95% test coverage
    And the quality scope test file patterns include "*_test.go"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage, crap
    Then the step passes
    And the command `go` is run with arguments:
      """
      tool
      cover
      -func=<artifact>/coverage-filtered.out
      """
