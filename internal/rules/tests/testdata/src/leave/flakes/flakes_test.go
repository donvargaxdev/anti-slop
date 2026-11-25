package flakes

import (
	"errors"
	"os"
	"strconv"
	"testing"
)

type fakeTest struct{}

func (fakeTest) Skip(string) {}

func (fakeTest) Ready() bool { return true }

func TestCases(t *testing.T) {
	if testing.Short() {
		t.Skip("long test")
	}
	if _, ok := os.LookupEnv("MODE"); !ok {
		t.Skip("MODE is missing")
	}
	for _, value := range []string{"a", "b"} {
		t.Run(value, func(t *testing.T) {
			if value == "" {
				t.Error("empty value")
			}
		})
	}
	for i := 0; i < 3; i++ {
		t.Log(i)
	}
	cases := []string{"a", "b"}
	for i := 0; i < len(cases); i++ {
		t.Run(cases[i], func(t *testing.T) {})
	}
	for i := range len(cases) {
		if !t.Run(cases[i], func(t *testing.T) {}) {
			t.Log("case failed")
		}
	}
	for _, name := range cases {
		if !t.Run(name, func(t *testing.T) {}) {
			t.Log("case failed")
		}
	}
}

func TestNumericCases(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {})
	}
	count := 3
	for i := range count {
		passed := t.Run(strconv.Itoa(i), func(t *testing.T) {})
		t.Log(i, passed)
	}
}

func TestStopOnStoredShortMode(t *testing.T) {
	for i := 0; i < 3; i++ {
		short := testing.Short()
		if short {
			break
		}
		t.Run("case", func(t *testing.T) {})
	}
}

func TestStopOnCopiedShortMode(t *testing.T) {
	for range 3 {
		short := testing.Short()
		var full = !short
		if short {
			t.Log("short mode")
		} else {
			full = short == false
		}
		switch full {
		case true:
			return
		}
	}
}

func TestSkipOnNilSwitchError(t *testing.T) {
	switch err := check(); err {
	case nil:
		t.Skip("optional")
	default:
		t.Log(err)
	}
}

func TestSkipInUnrelatedErrorSwitchCase(t *testing.T) {
	switch err := check(); err {
	case os.ErrInvalid:
	default:
		t.Skip("optional")
	}
}

func TestSkipAfterNilSwitchErrorFallsThrough(t *testing.T) {
	switch err := check(); err {
	case nil:
		fallthrough
	default:
		t.Skip("optional")
	}
}

func TestContinueToNextCase(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := validate(i); err != nil {
			t.Error(err)
			continue
		}
		t.Run(strconv.Itoa(i), func(t *testing.T) {})
	}
}

func validate(i int) error {
	if i == 1 {
		return os.ErrInvalid
	}
	return nil
}

func check() error { return nil }

func TestLoopConditionUsesFixedFailure(t *testing.T) {
	err := check()
	for attempts := 0; attempts < 3 && err != nil; attempts++ {
		t.Log(check())
	}
}

func TestLoopConditionDoesNotReceiveAnotherCheck(t *testing.T) {
	err := check()
	for ; err != nil; err = nil {
		err = check()
		t.Log(err)
	}
}

func TestLoopConditionIsNeverReachedWithUpdatedResult(t *testing.T) {
	err := check()
	for attempts := 0; attempts < 3 && err != nil; attempts++ {
		err = check()
		t.Log(err)
		break
	}
}

func TestFailureConditionWithoutRetry(t *testing.T) {
	for check() != nil {
		break
	}
	for !testing.Short() {
		break
	}
	for !testing.Short() && available() {
		t.Log("full run")
	}
	err := check()
	for i := 0; i < 3 && errors.Is(err, os.ErrInvalid); i = i + 1 {
		t.Log(i)
	}
	for t.Failed() {
		t.Log("already failed")
	}
}

func waitForReadiness(ready func() bool, state fakeTest) {
	for !ready() {
	}
	for !state.Ready() {
	}
}

func noRetryFromStoredOrUnassignedResult(state struct{ err error }, outcomes map[int]error) {
	err := check()
	for err != nil {
		var unused bool
		_ = unused
		err, _ = outcomes[0]
	}
	for err != nil {
		err = state.err
	}
}

func TestFailureAfterEmptyRange(t *testing.T) {
	failed := false
	for range 0 {
		failed = t.Failed()
	}
	if failed {
		t.Skip("optional")
	}
}

func skipOnIndependentField(t *testing.T, state struct{ optional bool }) {
	optional := state.optional
	if optional {
		t.Skip("optional")
	}
}

func TestFailure(t *testing.T) {
	if err := check(); err != nil {
		t.Fatal(err)
		fakeTest{}.Skip("not a testing method")
	}
}

func TestSkipOnShortOrFailure(t *testing.T) {
	if t.Failed() || testing.Short() {
		t.Skip("optional test")
	}
}

func TestIndependentChecks(t *testing.T) {
	err := check()
	other := err
	for i := 0; i < 3; i++ {
		other = check()
		if err == nil {
			break
		}
		if i > 0 {
			t.Log(other)
		}
		if testing.Short() {
			break
		}
	}

	state := struct{ err error }{err}
	for range 3 {
		if state.err == nil {
			break
		}
	}
}

func TestChecksWithoutRetryExits(t *testing.T) {
	for {
		if t.Name() != "" {
			break
		}
	}
	for range 3 {
		if available() {
			continue
		}
		if _, err := os.Stat("fixture"); err == nil {
			t.Log("fixture exists")
		}
		if check() != nil {
			// Check every iteration even when it fails.
		}
	}
}

func TestContinueWithConditionalSuccessExit(t *testing.T) {
	for i := range 3 {
		if err := validate(i); err != nil {
			t.Error(err)
			continue
		}
		t.Log("case passed")
		if testing.Short() {
			break
		}
	}
}

func TestSkipOnIndependentBranch(t *testing.T) {
	skip := false
	if testing.Short() {
		skip = t.Failed()
	} else if skip {
		t.Skip("optional test")
	}
}

func TestSkipOnOverwrittenOutcome(t *testing.T) {
	skip := t.Failed()
	if testing.Short() {
		skip = available()
	} else {
		skip = false
	}
	if skip {
		t.Skip("optional test")
	}
}

func TestSkipBeforeCleanupAssignsFailure(t *testing.T) {
	skip := testing.Short()
	t.Cleanup(func() { skip = t.Failed() })
	if skip {
		t.Skip("optional test")
	}
}

func TestSkipInDelayedCalls(t *testing.T) {
	if t.Failed() {
		defer func() { t.Skip("deferred") }()
		go func() { t.Skip("goroutine") }()
		t.Cleanup(func() { t.Skip("cleanup") })
		callback := func() { t.Skip("registered cleanup") }
		t.Cleanup(callback)
	}
}

func delayedSkip(t *testing.T) func() {
	if t.Failed() {
		return func() { t.Skip("later") }
	}
	return nil
}

func TestSkipInUnguardedImmediateCall(t *testing.T) {
	func() {
		if testing.Short() {
			t.Skip("optional test")
		}
	}()
}

func TestStopOnStoredFailure(t *testing.T) {
	outcome := t.Run("setup", func(t *testing.T) {})
	for i := range 3 {
		if outcome {
			t.Skip("setup passed")
		}
		outcome = !t.Run(strconv.Itoa(i), func(t *testing.T) {})
		if outcome {
			break
		}
	}
}

func available() bool { return true }

func retry(body any) {}

func TestRetryNonTestCallback(t *testing.T) {
	retry(func(value *int) { *value = 1 })
	retry(func(value int) int { return value + 1 })
}

func TestSkipAfterEverySwitchCaseClearsFailure(t *testing.T) {
	failed := t.Failed()
	switch testing.Short() {
	default:
		failed = false
	case true:
		failed = available()
	}
	if failed {
		t.Skip("optional test")
	}
}

func TestSkipInIndependentSwitchCase(t *testing.T) {
	var failed bool
	switch {
	case testing.Short():
		failed = t.Failed()
	default:
		if failed {
			t.Skip("optional test")
		}
	}
	_ = failed
}

func TestSkipAfterFallthroughClearsFailure(t *testing.T) {
	var failed bool
	switch {
	case testing.Short():
		failed = t.Failed()
		fallthrough
	default:
		failed = false
	}
	if failed {
		t.Skip("optional test")
	}
}

func skipAfterEverySelectClauseClearsFailure(t *testing.T, gate <-chan bool) {
	failed := t.Failed()
	select {
	case failed = <-gate:
		if failed {
			t.Skip("optional test")
		}
	case <-t.Context().Done():
		failed = false
	}
	if failed {
		t.Skip("optional test")
	}
}

func skipInIndependentSelectClause(t *testing.T, gate <-chan struct{}) {
	var failed bool
	select {
	case <-gate:
		failed = t.Failed()
	default:
		if failed {
			t.Skip("optional test")
		}
	}
	_ = failed
}

func TestSkipAfterGuaranteedLoops(t *testing.T) {
	failed := t.Failed()
	for range 1 {
		failed = false
	}
	if failed {
		t.Skip("optional test")
	}
	failed = t.Failed()
	for {
		failed = false
		break
	}
	if failed {
		t.Skip("optional test")
	}
}

func TestContinueEvenOnSuccess(t *testing.T) {
	for range 3 {
		if testing.Verbose() {
			t.Log("checking")
		} else if check() != nil {
			continue
		}
	}
	for range 3 {
		if check() != nil {
			continue
		}
		t.Log("passed")
		continue
	}
}

func TestRangeOverwritesStoredOutcome(t *testing.T) {
	failed := t.Failed()
	for _, failed = range [1]bool{false} {
		if failed {
			t.Skip("optional test")
		}
	}
	if failed {
		t.Skip("optional test")
	}

	failed = t.Failed()
	for failed = range map[bool]struct{}{false: {}} {
		if failed {
			t.Skip("optional test")
		}
	}
}

func TestCleanupReturnDoesNotExitLoop(t *testing.T) {
	for range 3 {
		t.Cleanup(func() {
			if check() == nil {
				return
			}
		})
	}
}

func TestContinueStillRunsPost(t *testing.T) {
	failed := false
	for first := true; first; failed = false {
		first = false
		failed = t.Failed()
		select {
		default:
			continue
		}
	}
	if failed {
		t.Skip("optional test")
	}
}

func TestSwitchBreakDoesNotExitLoop(t *testing.T) {
	failed := t.Failed()
	for range 1 {
		switch {
		default:
			break
		}
		failed = false
	}
	if failed {
		t.Skip("optional test")
	}
}

func TestSkipAfterReturningBranch(t *testing.T) {
	failed := t.Failed()
	if testing.Short() {
		failed = false
	} else {
		return
	}
	if failed {
		t.Skip("optional test")
	}
}

func TestFailureOnBreakDoesNotReachNextIteration(t *testing.T) {
	failed := false
	for range 3 {
		if failed {
			t.Skip("optional test")
		}
		if testing.Short() {
			failed = t.Failed()
			break
		}
	}
}

func TestNegatedOutcomeOnBreakDoesNotReachNextIteration(t *testing.T) {
	ok := true
	for i := range 3 {
		if !ok {
			t.Skip("optional test")
		}
		ok = t.Run(strconv.Itoa(i), func(t *testing.T) {})
		if !ok {
			break
		}
	}
}

func TestRangeOverwritesFailureFromPreviousIteration(t *testing.T) {
	failed := false
	for _, failed = range [3]bool{} {
		if failed {
			t.Skip("optional test")
		}
		failed = t.Failed()
	}
}

func TestFailureRecordedOnOnlyIteration(t *testing.T) {
	failed := false
	for range 1 {
		if failed {
			t.Skip("optional")
		}
		failed = t.Failed()
	}
}

func singleAttempt() {
	for range [1]int{} {
		if check() != nil {
			continue
		}
		break
	}
}

func TestComparedTestSuccess(t *testing.T) {
	if !t.Failed() == true {
		t.Skip("optional")
	}
}

func TestComparedSubtestSuccess(t *testing.T) {
	if false != t.Run("body", func(t *testing.T) {}) {
		t.Skip("optional")
	}
}

func TestSwitchBreakKeepsLoopRunning(t *testing.T) {
	for range 3 {
	local:
		switch {
		case t.Run("attempt", func(t *testing.T) {}):
			break local
		}
		switch {
		case t.Run("another attempt", func(t *testing.T) {}):
			if testing.Verbose() {
				break
			}
		}
	}
}

func TestSwitchRetainsIndependentExit(t *testing.T) {
	for range 3 {
		switch {
		case testing.Short(), t.Run("attempt", func(t *testing.T) {}):
			return
		}
	}
}

func TestSwitchSuccessCanContinue(t *testing.T) {
	for range 3 {
		switch {
		case t.Run("attempt", func(t *testing.T) {}):
			continue
		}
	}
}

func TestSwitchFallsThroughFromIndependentCase(t *testing.T) {
	for range 3 {
		switch {
		case testing.Short():
			fallthrough
		case t.Run("attempt", func(t *testing.T) {}):
			return
		}
	}
}

func TestComparedFailureLeavesNoFailedOutcome(t *testing.T) {
	failed := t.Failed()
	if false != failed {
		return
	}
	if failed {
		t.Skip("optional")
	}
}

func TestComparedSuccessLeavesNoFailedOutcome(t *testing.T) {
	ok := t.Run("body", func(t *testing.T) {})
	if ok == false {
		return
	}
	if !ok {
		t.Skip("optional")
	}
}

func TestFatalBranchDoesNotCarryFailure(t *testing.T) {
	failed := t.Failed()
	if testing.Short() {
		failed = false
	} else {
		t.Fatal("unsupported")
	}
	if failed {
		t.Skip("optional")
	}
}

func TestSkipBranchDoesNotCarryFailure(t *testing.T) {
	failed := t.Failed()
	switch {
	case testing.Short():
		t.SkipNow()
	default:
		failed = false
	}
	if failed {
		t.Skip("optional")
	}
}

func TestSkipOnIndependentSwitchAlternative(t *testing.T) {
	switch {
	case t.Failed(), testing.Short():
		t.Skip("optional")
	}
}

func TestSkipOnIndependentFallthrough(t *testing.T) {
	switch {
	case testing.Short():
		fallthrough
	case t.Failed():
		t.Skip("optional")
	}
}

func TestSkipInNestedFunctionOfFailureCase(t *testing.T) {
	switch {
	case t.Failed():
		t.Run("subtest", func(t *testing.T) {
			t.Skip("optional")
		})
	}
}

func TestSwitchReturnsOnFailure(t *testing.T) {
	failed := t.Failed()
	switch {
	case failed:
		return
	}
	if failed {
		t.Skip("optional")
	}
}

func TestSwitchDefaultAfterNonmatchingFailure(t *testing.T) {
	ok := t.Run("body", func(t *testing.T) {})
	switch ok {
	default:
		if !ok {
			t.Skip("optional")
		}
	case false:
		return
	}
}

func TestLaterCaseAfterNonmatchingFailure(t *testing.T) {
	failed := t.Failed()
	switch false {
	case !failed:
		return
	case !testing.Short():
		if failed {
			t.Skip("optional")
		}
	}
}

func TestSkipAfterOptionalFailurePath(t *testing.T) {
	if t.Failed() {
		t.Log("failed")
	}
	t.Skip("optional")
}

func TestSkipAfterOptionalSuccessPath(t *testing.T) {
	if !t.Failed() {
		t.Log("passed")
	}
	t.Skip("optional")
}

func TestSkipAfterSwitchWithIndependentBreak(t *testing.T) {
	switch {
	case testing.Short():
		break
	case !t.Failed():
		return
	}
	t.Skip("optional")
}

func TestSkipAfterSwitchWithIndependentMiss(t *testing.T) {
	switch {
	case t.Failed():
		t.Log("failed")
	}
	t.Skip("optional")
}

func TestImmediateCallClearsFailure(t *testing.T) {
	failed := t.Failed()
	func() { failed = false }()
	if failed {
		t.Skip("optional")
	}
}

func TestImmediateCallClearsFailureBeforeReturning(t *testing.T) {
	failed := t.Failed()
	func() {
		failed = false
		return
	}()
	if failed {
		t.Skip("optional")
	}
}

func TestCallbackReturnDoesNotResumeImmediateCaller(t *testing.T) {
	failed := t.Failed()
	func() {
		t.Cleanup(func() { return })
		failed = false
	}()
	if failed {
		t.Skip("optional")
	}
}

func TestAssignmentOverwritesFailureFromImmediateCall(t *testing.T) {
	failed := false
	failed = func() bool {
		failed = t.Failed()
		return false
	}()
	if failed {
		t.Skip("optional")
	}
}

func TestImmediateCallSeesArgumentSideEffects(t *testing.T) {
	failed := t.Failed()
	func(bool) {
		if failed {
			t.Skip("optional")
		}
	}(func() bool {
		failed = false
		return true
	}())
	if failed {
		t.Skip("optional")
	}
}

func TestImmediateCallRunsAfterTrueAnd(t *testing.T) {
	failed := t.Failed()
	_ = true && func() bool {
		failed = false
		return true
	}()
	if failed {
		t.Skip("optional")
	}
}

func TestAndClearsFailureWhenNeeded(t *testing.T) {
	failed := t.Failed()
	_ = failed && func() bool {
		failed = false
		return true
	}()
	if failed {
		t.Skip("optional")
	}
}

func TestOrClearsFailureWhenNeeded(t *testing.T) {
	passed := t.Run("setup", func(t *testing.T) {})
	_ = passed || func() bool {
		passed = true
		return true
	}()
	if !passed {
		t.Skip("optional")
	}
}

func TestOrEvaluatesRightOnlyOnSuccess(t *testing.T) {
	failed := t.Failed()
	_ = failed || func() bool {
		if failed {
			t.Skip("optional")
		}
		return false
	}()
}

func TestEnterLoopOnSuccess(t *testing.T) {
	passed := t.Run("setup", func(t *testing.T) {})
	for passed {
		if !passed {
			t.Skip("optional")
		}
		break
	}
}

func TestOptionalSkipAfterFailureLoop(t *testing.T) {
	for t.Failed() {
		break
	}
	t.Skip("optional")
}

func TestSkipAfterLoopThatClearsFailure(t *testing.T) {
	failed := t.Failed()
	for failed {
		failed = false
	}
	if failed {
		t.Skip("optional")
	}
}

func TestSkipAfterLoopThatReturnsOnFailure(t *testing.T) {
	failed := t.Failed()
	for failed {
		return
	}
	if failed {
		t.Skip("optional")
	}
}

func TestSkipAfterLoopThatRefreshesFailure(t *testing.T) {
	observed := t.Failed()
	failed := testing.Verbose()
	for failed {
		failed = observed
	}
	if failed {
		t.Skip("optional")
	}
}

func TestStopCheckingAtExpectedError(t *testing.T) {
	expected := errors.New("expected")
	for attempts := 0; attempts < 3 && check() != expected; attempts++ {
		t.Log("waiting for the expected error")
	}
}

func TestIndependentImmediateCalls(t *testing.T) {
	for i := range 3 {
		func() {
			if check() == nil {
				return
			}
			t.Log(i)
		}()
	}
}

func TestImmediateCallBindsLaterArgumentAfterSideEffects(t *testing.T) {
	failed := t.Failed()
	func(_ bool, failed bool) {
		if failed {
			t.Skip("optional")
		}
	}(func() bool {
		failed = false
		return true
	}(), failed && true)
}

func TestSkipInSuccessShortCircuit(t *testing.T) {
	_ = !t.Failed() && func() bool {
		t.Skip("optional")
		return true
	}()
	_ = t.Failed() || func() bool {
		t.SkipNow()
		return false
	}()
}

func TestFailFastLoop(t *testing.T) {
	for range 3 {
		if err := check(); err == nil {
			break
		}
		t.Fatal("failed")
	}
}

func TestFailFastSwitch(t *testing.T) {
attempts:
	for range 3 {
		switch err := check(); err {
		case nil:
			break attempts
		default:
			t.Fatal(err)
		}
	}
}

func TestFailFastCheckCanBeBypassed(t *testing.T) {
	for range 3 {
		if testing.Short() {
			continue
		}
		if err := check(); err == nil {
			break
		}
		t.Fatal("failed")
	}
}

func TestFailureContinuesAnOuterLoop(t *testing.T) {
outer:
	for range 3 {
		for range 3 {
			if err := check(); err == nil {
				break
			}
			continue outer
		}
	}
}

func TestFailFastInPost(t *testing.T) {
	for ; testing.Verbose(); t.FailNow() {
		if err := check(); err == nil {
			break
		}
	}
}

func TestSkipOnUnrelatedOutcomeReturnedByImmediateCall(t *testing.T) {
	if func() bool {
		_ = t.Failed()
		return testing.Short()
	}() {
		t.Skip("short mode")
	}
	passed := func() bool { return !t.Failed() }()
	if passed {
		t.Skip("passed")
	}
}

func TestSkipAfterLoopCanBreakOnSuccess(t *testing.T) {
	for !t.Failed() {
		break
	}
	t.Skip("optional")
}

func TestSkipAfterLoopConditionCanBecomeIndependent(t *testing.T) {
	passed := !t.Failed()
	for passed {
		passed = testing.Short()
	}
	t.Skip("optional")
}

func TestSkipAfterLoopConditionCallUsesUpdatedOutcome(t *testing.T) {
	passed := !t.Failed()
	for func() bool { return passed }() {
		passed = testing.Short()
	}
	t.Skip("optional")
}

func TestSelectOperandsClearFailureBeforeAnyBody(t *testing.T) {
	sink := make(chan bool, 1)
	failed := t.Failed()
	var replacement bool
	select {
	default:
		if failed {
			t.Skip("optional")
		}
	case replacement = <-func() <-chan bool {
		replacement = failed
		return nil
	}():
	case func() chan<- bool {
		failed = false
		return sink
	}() <- func() bool {
		replacement = false
		return false
	}():
		if replacement {
			t.Skip("optional")
		}
	}
	if failed || replacement {
		t.Skip("optional")
	}
}
