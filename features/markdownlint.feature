Feature: Markdownlint step
  The markdownlint step runs gomarklint using configured include/ignore rules.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the tool spec for "markdownlint" is "github.com/shinagawa-web/gomarklint/v3@v3.2.3"
    And markdownlint has an extra argument of "--config" specified
    And markdownlint has an extra argument of "features/testdata/markdown_bdd/.gomarklint.json" specified

  Scenario Outline: passes on clean markdown with matching local binary
    Given the local tool for "markdownlint" is "matching"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step passes
    And the tool runs locally
    And the command `gomarklint` is run with arguments:
      """
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean markdown with mismatched local binary
    Given the local tool for "markdownlint" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/shinagawa-web/gomarklint/v3@v3.2.3
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: passes on clean markdown with missing local binary
    Given the local tool for "markdownlint" is "missing"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step passes
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/shinagawa-web/gomarklint/v3@v3.2.3
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario: fails before dispatch with unprobeable local binary
    Given the local tool for "markdownlint" is "unprobeable"
    And the output mode is agent
    When the gate runs steps markdownlint
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario Outline: fails when markdownlint issues found with matching local binary
    Given the codebase has markdown violations
    And the local tool for "markdownlint" is "matching"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step fails
    And the tool runs locally
    And the command `gomarklint` is run with arguments:
      """
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when markdownlint issues found with mismatched local binary
    Given the codebase has markdown violations
    And the local tool for "markdownlint" is "mismatched"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/shinagawa-web/gomarklint/v3@v3.2.3
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when markdownlint issues found with missing local binary
    Given the codebase has markdown violations
    And the local tool for "markdownlint" is "missing"
    And the output mode is <mode>
    When the gate runs steps markdownlint
    Then the step fails
    And the tool runs via go run
    And the command `go` is run with arguments:
      """
      run
      github.com/shinagawa-web/gomarklint/v3@v3.2.3
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      """
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: fails when markdownlint issues found with unprobeable local binary
    Given the codebase has markdown violations
    And the local tool for "markdownlint" is "unprobeable"
    And the output mode is agent
    When the gate runs steps markdownlint
    Then the step fails
    And the output is an ERROR/Fix/Hint diagnostic
    And the tool is not available
    And the step does not dispatch any commands

  Scenario: passes markdownlint extra args through
    Given the local tool for "markdownlint" is "matching"
    And the output mode is agent
    And markdownlint has an extra argument of "--severity" specified
    And markdownlint has an extra argument of "error" specified
    When the gate runs steps markdownlint
    Then the step passes
    And the command `gomarklint` is run with arguments:
      """
      --config
      features/testdata/markdown_bdd/.gomarklint.json
      --severity
      error
      """
