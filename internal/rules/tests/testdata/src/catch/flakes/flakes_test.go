package flakes

import (
	"errors"
	test "testing"
)

func check() error { return errors.New("failure") }

func TestRetryUntilSwitchCaseSucceeds(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		switch {
		case t.Run("attempt", func(t *test.T) {}):
			return
		}
	}
}

func TestRetryUntilSwitchBreaksLoop(t *test.T) {
attempts:
	for range 3 { // want "retrying a check or test body"
		switch ok := t.Run("attempt", func(t *test.T) {}); ok {
		case true:
			break attempts
		}
	}
}

func TestRetryUntilSwitchDefaultSucceeds(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		switch {
		case !t.Run("attempt", func(t *test.T) {}):
			t.Log("retrying")
		default:
			return
		}
	}
}

func retryUntilSwitchHasNoError() {
	for range 3 { // want "retrying a check or test body"
		switch err := check(); err {
		case nil:
			return
		}
	}
}

func TestSkipOnNonNilSwitchError(t *test.T) {
	switch err := check(); err {
	case nil:
	default:
		t.Skip("failed") // want "skipping after a failure"
	}

	switch err := check(); err {
	case nil:
	case errors.New("another failure"):
		t.SkipNow() // want "skipping after a failure"
	}
}

func TestRetryAfterReplacingShortMode(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		passed := test.Short()
		passed = t.Run("attempt", body)
		if passed {
			return
		}
	}
}

func TestRetryAfterConditionallyReplacingShortMode(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		passed := test.Short()
		if test.Verbose() {
			passed = checkPassed()
		}
		if passed {
			return
		}
	}
}

func checkPassed() bool { return check() == nil }

func TestRetryUntilSwitchSuccessFallsThrough(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		switch false {
		case !t.Run("attempt", func(t *test.T) {}):
			fallthrough
		case t.Failed():
			return
		}
	}
}

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

func TestRetryCheckWithSuccessWork(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if err := check(); err != nil {
			continue
		}
		t.Log("check passed")
		return
	}
	for range 3 { // want "retrying a check or test body"
		if check() != nil {
			continue
		}
		t.Log("check passed")
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

func TestSkipOnConditionallyOverwrittenFailure(t *test.T) {
	failed := t.Failed()
	if test.Short() {
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnDeclaredFailure(t *test.T) {
	var failed = t.Failed()
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnFailureBeforeCleanup(t *test.T) {
	failed := t.Failed()
	t.Cleanup(func() { failed = false })
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnCapturedFailure(t *test.T) {
	failed := t.Failed()
	t.Cleanup(func() {
		if failed {
			t.Skip("failed") // want "skipping after a failure"
		}
	})
}

func TestSkipOnSwappedOutcome(t *test.T) {
	failed := t.Failed()
	other := false
	failed, other = other, failed
	if other {
		t.Skip("failed") // want "skipping after a failure"
	}
	if failed {
		t.Skip("independent condition")
	}
}

func TestRetryWithDeclaredFailure(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		var failed, _ = !t.Run("body", body), true
		if failed {
			continue
		}
		break
	}
}

func TestSkipOnMergedOutcomePolarities(t *test.T) {
	outcome := t.Failed()
	if test.Short() {
		outcome = t.Run("body", body)
	}
	if outcome {
		t.Skip("test failed") // want "skipping after a failure"
	} else {
		t.Skip("subtest failed") // want "skipping after a failure"
	}
}

func retrySubtest[T any](body func(*T))        {}
func retryTest[T, C any](ctx C, body func(*T)) {}

func TestGenericRetry(t *test.T) {
	retrySubtest[test.T](body)                 // want "retrying a test body"
	retryTest[test.T, string]("context", body) // want "retrying a test body"
}

func TestSkipAfterUnmatchedSwitch(t *test.T) {
	failed := t.Failed()
	switch {
	case test.Short():
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipAfterSwitchCases(t *test.T) {
	var failed bool
	switch {
	case test.Short():
		failed = t.Failed()
	default:
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipAfterUnmatchedTypeSwitch(t *test.T) {
	failed := t.Failed()
	switch any(t).(type) {
	case string:
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipAfterFallthrough(t *test.T) {
	var failed bool
	switch {
	case test.Short():
		failed = t.Failed()
		fallthrough
	default:
		if failed {
			t.Skip("failed") // want "skipping after a failure"
		}
	}
}

func skipAfterSelect(t *test.T, gate <-chan bool) {
	failed := t.Failed()
	select {
	case failed = <-gate:
	default:
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func skipAfterOptionalRange(t *test.T, count int) {
	failed := t.Failed()
	for range count {
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func skipAfterOptionalFor(t *test.T, count int) {
	failed := t.Failed()
	for i := 0; i < count; i++ {
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func retryUntilLabeledReturn() {
	for range 3 { // want "retrying a check or test body"
		if check() != nil {
			continue
		}
		if test.Verbose() {
			goto passed
		}
	passed:
		return
	}
}

func TestSkipAfterEarlyBreak(t *test.T) {
	failed := t.Failed()
	for range 1 {
		if test.Short() {
			break
		}
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipAfterEarlyContinue(t *test.T) {
	failed := t.Failed()
	for range 1 {
		if test.Short() {
			continue
		}
		failed = false
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func skipAfterLabeledBreak(t *test.T, retry bool) {
	failed := false
attempts:
	for ; retry; failed = false {
		switch {
		case test.Short():
			failed = t.Failed()
			break attempts
		default:
			failed = false
		}
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func skipAfterEmptyRange(t *test.T, values []bool) {
	failed := t.Failed()
	for _, failed = range values {
		if failed {
			t.Skip("optional test")
		}
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnFailureFromPreviousIteration(t *test.T) {
	failed := false
	for range 3 {
		retry(func() { t.Log("body") }) // want "retrying a test body"
		if failed {
			t.Skip("failed") // want "skipping after a failure"
		}
		failed = t.Failed()
	}
}

func skipInReturnedSubtest(t *test.T) bool {
	return t.Run("subtest", func(t *test.T) {
		if t.Failed() {
			t.Skip("failed") // want "skipping after a failure"
		}
	})
}

func TestSkipOnOutcomeWithNewPolarity(t *test.T) {
	ok := t.Run("subtest", func(t *test.T) {})
	for range 3 {
		if ok {
			t.Skip("failed") // want "skipping after a failure"
		}
		ok = !ok
	}
}

func TestSkipOnFailureFromContinueAndPost(t *test.T) {
	failed, previous := false, false
	for i := 0; i < 3; i, previous = i+1, failed {
		if previous {
			t.Skip("failed") // want "skipping after a failure"
		}
		if test.Short() {
			failed = t.Failed()
			continue
		}
		failed = false
	}
}

func TestFailureRecordedOnFinalIteration(t *test.T) {
	failed := false
	for range 1 {
		failed = t.Failed()
		continue
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnComparedFailure(t *test.T) {
	if t.Failed() == true {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnComparedSubtestFailure(t *test.T) {
	if t.Run("body", func(t *test.T) {}) == false {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestRetryUntilComparedSuccess(t *test.T) {
	for range 3 { // want "retrying a check or test body"
		if t.Run("body", func(t *test.T) {}) == true {
			break
		}
	}
}

func TestSkipOnStoredComparedFailure(t *test.T) {
	const succeeded = true
	failed := succeeded != t.Run("body", func(t *test.T) {})
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnSwitchFailure(t *test.T) {
	switch {
	case t.Failed():
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnBooleanSwitchFailure(t *test.T) {
	switch ok := t.Run("body", func(t *test.T) {}); ok {
	case false:
		t.SkipNow() // want "skipping after a failure"
	}
}

func TestSkipOnDefaultFailure(t *test.T) {
	switch {
	default:
		t.Skip("failed") // want "skipping after a failure"
	case !t.Failed():
		t.Log("passed")
	}
}

func TestSkipOnFailureWithConstantSwitchTag(t *test.T) {
	switch false {
	case !t.Failed():
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnFailureWithFallthrough(t *test.T) {
	switch {
	case t.Failed() && test.Short():
		fallthrough
	case t.Failed():
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnFailureAfterLoggingError(t *test.T) {
	failed := t.Failed()
	if test.Short() {
		t.Error("unsupported")
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSkipOnFailureAfterDeferredFatal(t *test.T) {
	failed := t.Failed()
	defer t.Fatal("unsupported")
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSwitchReturnsOnSuccess(t *test.T) {
	failed := t.Failed()
	switch failed {
	case false:
		return
	}
	if failed {
		t.Skip("failed") // want "skipping after a failure"
	}
}

func TestSwitchAlternativeKeepsFailureOutcome(t *test.T) {
	failed := t.Failed()
	switch {
	case !failed, test.Short():
		if failed {
			t.Skip("failed") // want "skipping after a failure"
		}
	}
}

func TestFallthroughCarriesFailureToLaterBody(t *test.T) {
	failed := t.Failed()
	switch {
	case failed && test.Short():
		fallthrough
	case !failed:
		if failed {
			t.Skip("failed") // want "skipping after a failure"
		}
	}
}
