Feature: Format step
  The format step applies configured formatters over the configured scope.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a lint config path of ".golangci.yml"
    And the tool spec for "format" is "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

  Scenario Outline: passes on clean code with matching local binary
    Given the codebase has no lint violations
    And the local tool for "format" is "matching"
    And the output mode is <mode>
    When the gate runs steps format
    Then the step passes
    And the tool runs locally
    And the command `golangci-lint` is run with arguments:
      """
      fmt
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean code with mismatched local binary
    Given the codebase has no lint violations
    And the local tool for "format" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps format
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      fmt
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean code with missing local binary
    Given the codebase has no lint violations
    And the local tool for "format" is "missing"
    And the output mode is <mode>
    When the gate runs steps format
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      fmt
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario: fails before dispatch with unprobeable local binary
    Given the codebase has no lint violations
    And the local tool for "format" is "unprobeable"
    And the output mode is agent
    When the gate runs steps format
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario: passes format extra args through
    Given the codebase has no lint violations
    And the local tool for "format" is "matching"
    And the output mode is agent
    And lint has an extra argument of "arg1" specified
    When the gate runs steps format
    Then the step passes
    And the command `golangci-lint` is run with arguments:
      """
      fmt
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      arg1
      """

  Scenario: uses the configured package scope
    Given the codebase has no lint violations
    And the local tool for "format" is "matching"
    And the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps format
    Then the step passes
    And the command `golangci-lint` is run with arguments:
      """
      fmt
      -c
      .golangci.yml
      ./cmd/...
      """
