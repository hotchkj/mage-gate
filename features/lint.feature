Feature: Lint step
  The lint step runs golangci-lint against the configured scope.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a lint config path of ".golangci.yml"
    And the tool spec for "lint" is "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

  Scenario Outline: passes on clean code with matching local binary
    Given the codebase has no lint violations
    And the local tool for "lint" is "matching"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step passes
    And the tool runs locally
    And the command `golangci-lint` is run with arguments:
      """
      run
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
    And the local tool for "lint" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      run
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
    And the local tool for "lint" is "missing"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      run
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
    And the local tool for "lint" is "unprobeable"
    And the output mode is agent
    When the gate runs steps lint
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario Outline: fails when violations found with matching local binary
    Given the codebase has lint violations
    And the local tool for "lint" is "matching"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step fails
    And the tool runs locally
    And the command `golangci-lint` is run with arguments:
      """
      run
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when violations found with mismatched local binary
    Given the codebase has lint violations
    And the local tool for "lint" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      run
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when violations found with missing local binary
    Given the codebase has lint violations
    And the local tool for "lint" is "missing"
    And the output mode is <mode>
    When the gate runs steps lint
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
      run
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: fails when violations found with unprobeable local binary
    Given the codebase has lint violations
    And the local tool for "lint" is "unprobeable"
    And the output mode is agent
    When the gate runs steps lint
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario: passes lint extra args through
    Given the codebase has no lint violations
    And the local tool for "lint" is "matching"
    And the output mode is agent
    And lint has an extra argument of "arg1" specified
    When the gate runs steps lint
    Then the step passes
    And the command `golangci-lint` is run with arguments:
      """
      run
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      arg1
      """

  Scenario: uses the configured package scope
    Given the codebase has no lint violations
    And the local tool for "lint" is "matching"
    And the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps lint
    Then the step passes
    And the command `golangci-lint` is run with arguments:
      """
      run
      -c
      .golangci.yml
      ./cmd/...
      """
