Feature: Deadcode step
  The deadcode step runs deadcode analysis to detect unreachable code.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the tool spec for "deadcode" is "golang.org/x/tools/cmd/deadcode@v0.31.0"

  Scenario Outline: passes on clean code with matching local binary
    Given the local tool for "deadcode" is "matching"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step passes
    And the tool runs locally
    And the command `deadcode` is run with arguments:
      """
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean code with mismatched local binary
    Given the local tool for "deadcode" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      golang.org/x/tools/cmd/deadcode@v0.31.0
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean code with missing local binary
    Given the local tool for "deadcode" is "missing"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      golang.org/x/tools/cmd/deadcode@v0.31.0
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario: fails before dispatch with unprobeable local binary
    Given the local tool for "deadcode" is "unprobeable"
    And the output mode is agent
    When the gate runs steps deadcode
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario Outline: fails when deadcode issues found with matching local binary
    Given the codebase has dead code
    And the local tool for "deadcode" is "matching"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step fails
    And the tool runs locally
    And the command `deadcode` is run with arguments:
      """
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when deadcode issues found with mismatched local binary
    Given the codebase has dead code
    And the local tool for "deadcode" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      golang.org/x/tools/cmd/deadcode@v0.31.0
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when deadcode issues found with missing local binary
    Given the codebase has dead code
    And the local tool for "deadcode" is "missing"
    And the output mode is <mode>
    When the gate runs steps deadcode
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      golang.org/x/tools/cmd/deadcode@v0.31.0
      ./zz_bdd_not_default/...
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: fails when deadcode issues found with unprobeable local binary
    Given the codebase has dead code
    And the local tool for "deadcode" is "unprobeable"
    And the output mode is agent
    When the gate runs steps deadcode
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario: passes deadcode extra args through
    Given the local tool for "deadcode" is "matching"
    And the output mode is agent
    And deadcode has an extra argument of "arg1" specified
    When the gate runs steps deadcode
    Then the step passes
    And the command `deadcode` is run with arguments:
      """
      arg1
      ./zz_bdd_not_default/...
      """

  Scenario: uses the configured package scope
    Given the local tool for "deadcode" is "matching"
    And the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps deadcode
    Then the step passes
    And the command `deadcode` is run with arguments:
      """
      ./cmd/...
      """
