package flakes

import (
	"errors"
	test "testing"
)

func check() error { return errors.New("failure") }

func TestRetryCheck(t *test.T) {
	for attempt := 0; attempt < 3; attempt++ { // want "retrying a check or test body"
		if err := check(); err == nil {
			return
		}
	}
	t.Fatal("check failed")
}

func TestRetryHeader(t *test.T) {
	for err := check(); err != nil; err = check() { // want "retrying a check or test body"
		t.Log(err)
	}
}

func TestRetryAssignedCheck(t *test.T) {
	var err error
	for range 3 { // want "retrying a check or test body"
		err = check()
		if err != nil {
			continue
		}
		return
	}
	t.Fatal(err)
}

func TestRetrySubtest(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if t.Run("body", func(t *test.T) { t.Fatal("failure") }) {
			break
		}
	}
}

func TestRetryStoredSubtest(t *test.T) {
	for i := 0; i < 3; i++ { // want "retrying a check or test body"
		passed := t.Run("body", func(t *test.T) { t.Fatal("failure") })
		if !passed {
			continue
		}
		return
	}
}

func TestSkipOnFailure(t *test.T) {
	if err := check(); err != nil {
		t.Skipf("failed: %v", err) // want "skipping after a failure"
	}
	if t.Failed() {
		t.SkipNow() // want "skipping after a failure"
	}
	if !t.Run("body", func(t *test.T) { t.Fatal("failure") }) {
		t.Skip("failure") // want "skipping after a failure"
	}
	passed := t.Run("another body", func(t *test.T) { t.Fatal("failure") })
	if !passed {
		t.Skip("failure") // want "skipping after a failure"
	}
	if failed := t.Failed(); failed {
		t.Skip("failure") // want "skipping after a failure"
	}
}

func retry(fn func()) { fn() }

func TestRetryCallback(t *test.T) {
	retry(func() { t.Fatal("check") }) // want "retrying a test body"
}

func retryTest(t *test.T, body func(*test.T)) {
	body(t)
}

func body(t *test.T) { t.Fatal("check") }

func TestRetryNamedCallback(t *test.T) {
	retryTest(t, body) // want "retrying a test body"
}
