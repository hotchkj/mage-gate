Feature: Config validation
  Invalid or missing configuration is rejected before commands run meaningfully.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"

  Scenario: missing coverage threshold is rejected
    When the gate runs steps coverage
    Then the configuration is rejected

  Scenario: coverage threshold above 100 is rejected
    Given a coverage threshold of 110
    When the gate runs steps coverage
    Then the configuration is rejected

  Scenario: zero coverage threshold disables the check
    Given a coverage threshold of 0
    And the codebase has 10% test coverage
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
    Then the step passes
    And the output is empty

  Scenario: missing CRAP threshold is rejected
    When the gate runs steps crap
    Then the configuration is rejected

  Scenario: zero CRAP threshold is rejected
    Given a CRAP threshold of 0
    When the gate runs steps crap
    Then the configuration is rejected

  Scenario: missing lint config path is rejected
    When the gate runs steps lint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: missing lint tool spec is rejected
    Given a lint config path of ".golangci.yml"
    When the gate runs steps lint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: missing deadcode tool spec is rejected
    When the gate runs steps deadcode
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: missing markdownlint tool spec is rejected
    When the gate runs steps markdownlint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: malformed lint tool spec is rejected
    Given a lint config path of ".golangci.yml"
    And the tool spec for "lint" is "not a valid spec"
    When the gate runs steps lint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: custom golangci-lint requires an explicit builder tool spec
    Given a lint config path of ".golangci.yml"
    And the tool spec for "lint" is "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
    And a custom golangci-lint definition path of ".custom-gcl.yml"
    When the gate runs steps lint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: malformed deadcode tool spec is rejected
    Given the tool spec for "deadcode" is "not a valid spec"
    When the gate runs steps deadcode
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: malformed markdownlint tool spec is rejected
    Given the tool spec for "markdownlint" is "not a valid spec"
    When the gate runs steps markdownlint
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: missing gocyclo tool spec is rejected
    When the gate runs steps crap
    Then the configuration is rejected
    And the step does not dispatch any commands

  Scenario: missing gremlins tool spec is rejected for mutation scan
    Given a mutation threshold of 2 sites
    When the gate runs steps qualityscopeinventory, mutationscan
    Then the configuration is rejected

  Scenario: missing gremlins tool spec is rejected for mutationkills
    Given a mutation kills min rate of 80 percent
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the configuration is rejected

  Scenario: missing mutation kills min rate is rejected
    Given the tool spec for "mutationkills" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
    And the mutation test result has 8 killed and 2 lived mutations
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the configuration is rejected
