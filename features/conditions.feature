@phase-1
Feature: Compound conditions
  anti-slop conditions <pattern>... reports if and for conditions with more than two
  logical operators. Each finding occupies one line on stdout and the command exits 0.

  @ID-CONDITIONS-01 @issue-13 @binary-only
  Scenario: conditions reports long if and for expressions and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func check(a, b, c, d bool) {
          if a && b && c && d {}
          for a || (b && c) || d { break }
      }
      """
    When anti-slop runs with "conditions ./..."
    Then it exits with code 0
    And its standard output lists "p.go:4:8: conditions: split this condition into named booleans"
    And its standard output lists "p.go:5:9: conditions: split this condition into named booleans"
    And its error output is empty

  @ID-CONDITIONS-02 @issue-13 @binary-only
  Scenario: conditions leaves short expressions and named booleans alone
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func check(a, b, c, d bool) {
          named := a && b && c && d
          if named || b && c {}
          for named { break }
      }
      """
    When anti-slop runs with "conditions ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-CONDITIONS-03 @issue-13
  Scenario: conditions refuses package patterns that look like flags
    When anti-slop runs with "conditions -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-CONDITIONS-04 @issue-13 @binary-only
  Scenario: conditions reports when no packages can be loaded
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "conditions ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"
