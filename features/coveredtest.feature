Feature: CoveredTest step
  The covered test step runs go test once with coverage instrumentation for the configured scope.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"

  Scenario Outline: passes when tests succeed with coverage instrumentation
    Given the codebase has 95% test coverage
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when tests fail
    Given the codebase has failing tests
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: passes extra args through to the underlying test command
    Given the output mode is agent
    And coveredtest has an extra argument of "arg1" specified
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step passes
    And the command `go` is run with arguments:
      """
      test
      ./zz_bdd_not_default/...
      -json
      -coverprofile=<artifact>/coverage.out
      -coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil
      arg1
      -count=1
      """

  Scenario: uses the configured package scope as run target
    Given the codebase has 95% test coverage
    And the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step passes
    And the command `go` is run with arguments:
      """
      test
      ./cmd/...
      -json
      -coverprofile=<artifact>/coverage.out
      -coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil
      -count=1
      """

  Scenario: uses the configured quality scope for measurement
    Given the codebase has 95% test coverage
    And the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step passes
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./internal/...
      """

  Scenario: quality scope excludes narrow the measurement boundary
    Given the codebase has 95% test coverage
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest
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

  Scenario: quality-scope tags affect covered test instrumentation
    Given the codebase has 95% test coverage
    And the quality scope has build tag "mage"
    And the quality scope has build tag "integration"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step passes
    And the command `go` is run with arguments:
      """
      test
      ./zz_bdd_not_default/...
      -json
      -coverprofile=<artifact>/coverage.out
      -coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil
      -tags=mage,integration
      -count=1
      """

  Scenario: covered test rejects generic build-tag arguments
    Given coveredtest has an extra argument of "-tags=consumer" specified
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the step fails
    And the error is ErrInvalidOption
    And the step does not dispatch any commands

