Feature: Coverage step
  The coverage step checks statement coverage against a configured threshold.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a coverage threshold of 90
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"

  Scenario Outline: passes when above threshold
    Given the codebase has 95% test coverage
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when below threshold
    Given the codebase has 85% test coverage
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: uses the configured quality scope for measurement
    Given the codebase has 95% test coverage
    And the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
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
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
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

  Scenario: unfiltered quality scope uses the raw coverage profile
    Given the codebase has 95% test coverage
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
    Then the step passes
    And the command `go` is run with arguments:
      """
      tool
      cover
      -func=<artifact>/coverage.out
      """
