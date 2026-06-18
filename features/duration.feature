Feature: Duration step
  The duration step verifies that no test exceeds the configured time threshold.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a duration threshold of 5 seconds

  Scenario Outline: passes when tests are fast
    Given the output mode is <mode>
    When the gate runs steps test, duration
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario: passes when package wall-clock exceeds the threshold but each test is fast
    Given the codebase has fast tests with slow package wall-clock
    And a duration threshold of 1 seconds
    And the output mode is agent
    When the gate runs steps test, duration
    Then the step passes
    And the output is empty

  Scenario Outline: fails when tests exceed the duration threshold
    Given the codebase has slow tests
    And a duration threshold of 1 seconds
    And the output mode is <mode>
    When the gate runs steps test, duration
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: duration checks all test events from the producing test run
    Given the codebase has 95% test coverage
    And the quality scope excludes "testutil"
    And the output mode is agent
    And the duration threshold is 1.0 seconds
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
    And package "example.com/mod/zz_bdd_not_default/testutil" has a test event "TestSlowUtility" lasting 2.0 seconds
    When the gate runs steps qualityscopeinventory, coveredtest, duration
    Then the step fails
    And the error is ErrDurationFailed
