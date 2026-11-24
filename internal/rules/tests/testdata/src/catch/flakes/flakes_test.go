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

func TestRetryCheckWithConditionalLogging(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if err := check(); err == nil {
			return
		}
		if test.Verbose() {
			t.Log("retrying")
		}
	}
	t.Fatal("check failed")
}

func TestRetryWithCollectionBound(t *test.T) {
	backoffs := []int{1, 2, 3}
	for i := 0; i < len(backoffs); i++ { // want "retrying a check or test body"
		if err := check(); err == nil {
			return
		}
	}
	t.Fatal("check failed")
}

func TestRetryWithCollectionSizedRange(t *test.T) {
	backoffs := []int{1, 2, 3}
	for range len(backoffs) { // want "retrying a check or test body"
		if err := check(); err == nil {
			return
		}
	}
	t.Fatal("check failed")
}

func TestRetryWithCollectionRange(t *test.T) {
	backoffs := []int{1, 2, 3}
	for range backoffs { // want "retrying a check or test body"
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

func TestRetryCheckUntilBreak(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if err := check(); err != nil {
			continue
		}
		break
	}
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

func TestRetryStoredFailure(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		failed := !t.Run("body", body)
		if !failed {
			break
		}
	}
}

func TestRetryFailureStoredInIfInit(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		passed := t.Run("body", body)
		if passed = !passed; passed {
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

func body(t *test.T) { t.Fatal("check") }

func probe() bool { return true }

func TestRetryUntilCheckPasses(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if probe() {
			break
		}
	}
	for range 3 { // want "retrying a check or test body"
		if check() == nil {
			break
		}
	}
}

func TestSkipOnCompoundFailure(t *test.T) {
	if t.Failed() && !test.Short() {
		t.Skip("failed outside short mode") // want "skipping after a failure"
	}
	if t.Failed() || check() != nil {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func retrySubtest[T any](body func(*T))        {}
func retryTest[T, C any](ctx C, body func(*T)) {}

func TestGenericRetry(t *test.T) {
	retrySubtest[test.T](body)                 // want "retrying a test body"
	retryTest[test.T, string]("context", body) // want "retrying a test body"
}
