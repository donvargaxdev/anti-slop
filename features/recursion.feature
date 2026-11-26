@phase-1
Feature: Recursion
  anti-slop recursion <pattern>... reports named functions and methods that call
  themselves through one to three statically resolved calls in the same package.
  Each finding occupies one line on stdout and the command exits 0.

  @ID-RECURSION-01 @recursion-rule @binary-only
  Scenario: recursion reports each function in a cycle and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func direct() { direct() }
      func first() { second() }
      func second() { first() }
      func caller() { first() }
      """
    When anti-slop runs with "recursion ./..."
    Then it exits with code 0
    And its standard output lists "p.go:3:6: recursion: direct calls itself"
    And its standard output lists "p.go:4:6: recursion: first calls itself through a cycle"
    And its standard output lists "p.go:5:6: recursion: second calls itself through a cycle"
    And its standard output does not say "caller"
    And its error output is empty

  @ID-RECURSION-02 @recursion-rule @binary-only
  Scenario: recursion leaves a call chain that ends
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func first() { second() }
      func second() {}
      """
    When anti-slop runs with "recursion ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-RECURSION-03 @recursion-rule
  Scenario: recursion rejects a pattern that go list would take for a flag
    When anti-slop runs with "recursion -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-RECURSION-04 @recursion-rule @binary-only
  Scenario: recursion reports when packages cannot be loaded
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "recursion ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"
