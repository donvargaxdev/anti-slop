@phase-1
Feature: Test reliability
  anti-slop tests reports sleeps, wall-clock polling, process environment changes and
  flake masking in Go test files. Findings go to stdout and leave the exit code at 0.

  @ID-TESTS-01 @issue-3 @binary-only
  Scenario: tests reports each kind of finding and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      import "time"

      func Production() { time.Sleep(time.Second) }
      """
    And the file "p_test.go" holds:
      """
      package p

      import (
          "os"
          "testing"
          "time"
      )

      func pause() {
          time.Sleep(time.Second)
      }

      func TestProbe(t *testing.T) {
          pause()
          started := time.Now()
          for {
              if time.Since(started) > time.Second {
                  break
              }
          }
          _ = os.Setenv("MODE", "test")
          if t.Failed() {
              t.Skip("try again later")
          }
      }

      func TestTimeout(t *testing.T) {
          started := time.Now()
          for {
              elapsed := time.Since(started)
              if elapsed > time.Second {
                  t.Fatal("timeout")
              }
          }
      }

      func TestSelect(t *testing.T) {
          gate := make(chan struct{})
          failed := t.Failed()
          select {
          case <-gate:
              failed = false
          default:
          }
          if failed {
              t.Skip("failed")
          }
      }

      func TestTimeoutGuard(t *testing.T) {
          started := time.Now()
          for {
              if time.Since(started) < time.Second {
                  continue
              }
              t.Fatal("timeout")
          }
      }

      func pollHeader(ready func() bool) {
          for (<-time.After(time.Millisecond)).IsZero() {
          }
          for ; !ready(); <-time.After(time.Millisecond) {
          }
      }

      func TestSkipAfterGuard(t *testing.T) {
          if !t.Failed() {
              return
          }
          t.Skip("failed")
      }

      func TestSkipAfterSwitchGuard(t *testing.T) {
          switch {
          case !t.Failed():
              return
          }
          t.Skip("failed")
      }

      func TestRetryInLoopBody(t *testing.T) {
          err := check()
          for attempts := 0; attempts < 3 && err != nil; attempts++ {
              err = check()
          }
      }

      func check() error { return nil }

      func TestRetryInLoopCondition(t *testing.T) {
          for attempts := 0; attempts < 3 && check() != nil; attempts++ {
          }
      }

      func TestSkipInImmediateCall(t *testing.T) {
          if t.Failed() {
              func() {
                  t.Skip("failed")
              }()
          }
      }

      func TestSkipInShortCircuit(t *testing.T) {
          _ = t.Failed() && func() bool {
              t.Skip("failed")
              return true
          }()
          _ = !t.Failed() || func() bool {
              t.SkipNow()
              return false
          }()
      }
      """
    When anti-slop runs with "tests ./..."
    Then it exits with code 0
    And its standard output is:
      """
      p_test.go:10:5: tests: time.Sleep in test code
      p_test.go:16:5: tests: polling against the wall clock
      p_test.go:21:9: tests: os.Setenv changes the process environment in test code
      p_test.go:23:9: tests: skipping after a failure
      p_test.go:29:5: tests: polling against the wall clock
      p_test.go:46:9: tests: skipping after a failure
      p_test.go:52:5: tests: polling against the wall clock
      p_test.go:61:5: tests: polling against the wall clock
      p_test.go:63:5: tests: polling against the wall clock
      p_test.go:71:5: tests: skipping after a failure
      p_test.go:79:5: tests: skipping after a failure
      p_test.go:84:5: tests: retrying a check or test body
      p_test.go:92:5: tests: retrying a check or test body
      p_test.go:99:13: tests: skipping after a failure
      p_test.go:106:9: tests: skipping after a failure
      p_test.go:110:9: tests: skipping after a failure
      """
    And its error output is empty

  @ID-TESTS-02 @issue-3 @binary-only
  Scenario: tests prints nothing for cleanup, synchronization and distinct subtests
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p
      """
    And the file "p_test.go" holds:
      """
      package p_test

      import (
          "strconv"
          "testing"
      )

      func TestProbe(t *testing.T) {
          if testing.Short() {
              t.Skip("requires the full test run")
          }
          t.Setenv("MODE", "test")
          ready := make(chan struct{})
          close(ready)
          <-ready
          for i := 0; i < 3; i++ {
              t.Run(strconv.Itoa(i), func(t *testing.T) {})
          }
      }

      func TestHandledFailure(t *testing.T) {
          failed := t.Failed()
          switch {
          case failed:
              return
          }
          if failed {
              t.Skip("optional")
          }
      }

      func TestFailFast(t *testing.T) {
          for range 3 {
              if err := check(); err == nil {
                  break
              }
              t.Fatal("failed")
          }
      }

      func TestHandledError(t *testing.T) {
          err := check()
          if err != nil {
              err = nil
          }
          if err != nil {
              t.Skip("optional")
          }
      }

      func check() error { return nil }
      """
    When anti-slop runs with "tests ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-TESTS-03 @issue-3
  Scenario: tests requires a package pattern
    When anti-slop runs with "tests"
    Then it exits with code 80
    And its error output says "no package is named"

  @ID-TESTS-04 @issue-3
  Scenario: tests refuses a pattern that go list would take for a flag
    When anti-slop runs with "tests -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-TESTS-05 @issue-3 @binary-only
  Scenario: tests reports when a pattern matches no package
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "tests ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"
