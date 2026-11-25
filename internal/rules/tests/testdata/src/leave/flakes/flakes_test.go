package flakes

import (
	"os"
	"strconv"
	"testing"
)

type fakeTest struct{}

func (fakeTest) Skip(string) {}

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
