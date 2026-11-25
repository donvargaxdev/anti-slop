@phase-1
Feature: Test smells
  anti-slop tests <pattern>... reports calls to time.Sleep, os.Setenv and os.Unsetenv
  in Go test files. Each finding occupies one line on stdout and the command exits 0.

  @ID-TESTS-01 @tests-rule @binary-only
  Scenario: tests reports sleep and raw environment changes and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p_test.go" holds:
      """
      package p

      import (
          "os"
          "time"
      )

      func change() {
          time.Sleep(time.Second)
          _ = os.Setenv("EXAMPLE", "value")
          _ = os.Unsetenv("EXAMPLE")
      }
      """
    When anti-slop runs with "tests ./..."
    Then it exits with code 0
    And its standard output lists "p_test.go:9:5: tests: avoid time.Sleep in tests"
    And its standard output lists "p_test.go:10:9: tests: use t.Setenv instead of os.Setenv"
    And its standard output lists "p_test.go:11:9: tests: avoid os.Unsetenv in tests"
    And its error output is empty

  @ID-TESTS-02 @tests-rule @binary-only
  Scenario: tests leaves t.Setenv and calls in ordinary Go files
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      import "time"

      func wait() { time.Sleep(time.Second) }
      """
    And the file "p_test.go" holds:
      """
      package p_test

      import "testing"

      func TestEnvironment(t *testing.T) {
          t.Setenv("EXAMPLE", "value")
      }
      """
    When anti-slop runs with "tests ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-TESTS-03 @tests-rule
  Scenario: tests rejects a pattern that go list would take for a flag
    When anti-slop runs with "tests -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-TESTS-04 @tests-rule @binary-only
  Scenario: tests reports when packages cannot be loaded
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "tests ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"
