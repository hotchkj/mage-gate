Feature: Step failure propagation
  When a step fails, subsequent steps do not execute.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a lint config path of ".golangci.yml"
    And the tool spec for "lint" is "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
    And the output mode is agent

  Scenario: gate stops at first failure with matching local lint binary
    Given the codebase has lint violations
    And the local tool for "lint" is "matching"
    When the gate runs steps lint, compile, test
    Then the step fails
    And the following steps do not run: "compile", "test"
    And the tool runs locally
    And the command `golangci-lint` is run with arguments:
      """
      run
      -c
      .golangci.yml
      ./zz_bdd_not_default/...
      """

  Scenario: gate stops at first failure with go run lint fallback
    Given the codebase has lint violations
    And the local tool for "lint" is "missing"
    When the gate runs steps lint, compile, test
    Then the step fails
    And the following steps do not run: "compile", "test"
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
