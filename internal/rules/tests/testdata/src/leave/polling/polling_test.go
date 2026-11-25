package polling

import (
	"testing"
	"time"
)

func TestAdjustedFixedDuration(t *testing.T) {
	start := time.Now()
	for {
		elapsed := time.Since(start)
		elapsed = time.Second
		elapsed += time.Millisecond
		if elapsed > time.Second {
			t.Fatal("timeout")
		}
	}
}

func TestShortDeclarationReplacesClockValue(t *testing.T) {
	start := time.Now()
	for {
		elapsed := time.Since(start)
		elapsed, limit := time.Second, 2*time.Second
		if elapsed > limit {
			t.Fatal("timeout")
		}
	}
}

type fakeClock struct{}

func (fakeClock) Now() bool { return false }

type fakeTest struct{}

func (fakeTest) Fatal(string) {}

func TestPoll(t *testing.T) {
	start := time.Now()
	for range 3 {
		t.Log(time.Since(start))
	}
	for clock := (fakeClock{}); clock.Now(); {
		t.Log("uses a fake clock")
	}
	ready := make(chan struct{})
	close(ready)
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestTimingDoesNotCauseFailureExit(t *testing.T) {
	start := time.Now()
	for range 3 {
		if time.Since(start) > time.Second {
			fakeTest{}.Fatal("slow")
			t.Error("slow")
		}
		if t.Failed() {
			t.FailNow()
		}
	}
}

func TestTimingOnlyForFailureMessage(t *testing.T) {
	start := time.Now()
	for range 3 {
		if elapsed, failed := time.Since(start), t.Failed(); failed {
			t.Fatalf("failed after %v", elapsed)
		}
	}
}

func TestClosureDoesNotReadClockInCondition(t *testing.T) {
	for condition(func() time.Time { return time.Now() }) {
		break
	}
}

func condition(func() time.Time) bool { return false }

func immediatePredicateWithDeferredClockCheck() {
	for func() bool {
		defer func() { _ = time.Now() }()
		return condition(func() time.Time { return time.Now() })
	}() {
	}
}

func immediatePredicateWithUnusedClockSample() {
	for func() bool {
		_ = func() time.Time { return time.Now() }()
		return testing.Short()
	}() {
	}
}

func immediatePredicateWithFixedSnapshot() {
	now := time.Now()
	for func() bool { return now.IsZero() }() {
	}
}

func TestImmediatePredicateWithClockArgumentOnlyForLogging(t *testing.T) {
	for func(now time.Time) bool {
		t.Log(now)
		return testing.Short()
	}(time.Now()) {
		break
	}
}

func TestImmediatePredicateWithClockGuardOnlyForLogging(t *testing.T) {
	start := time.Now()
	for func() bool {
		if time.Since(start) > time.Second {
			t.Log("slow")
		}
		return testing.Short()
	}() {
		break
	}
}

func receiveReadyChannelReturnedByImmediateCall() {
	ready := make(chan time.Time)
	close(ready)
	for range 3 {
		<-func() <-chan time.Time {
			_ = time.After(time.Millisecond)
			return ready
		}()
	}
}

func TestClockSnapshotAndUnrelatedRefresh(t *testing.T) {
	end := time.Now()
	for now := time.Now(); now.Before(end); now = now.Add(time.Second) {
		t.Log(now)
	}
	var sampled time.Time
	for i := 0; i < 3; i, sampled = i+1, time.Now() {
		t.Log(sampled)
	}
}

func receiveDeadlineDuringInitialization(ready func() bool) {
	for <-time.After(time.Millisecond); !ready(); {
	}
}

func receiveOnLoopExit() {
	for ; ; <-time.After(time.Millisecond) {
		break
	}
}

func receiveReadinessInHeader(ready <-chan time.Time, done func() bool) {
	for (<-ready).IsZero() {
	}
	for ; !done(); <-ready {
	}
}

func TestDeadlineReassignedToReady(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Millisecond)
	<-ch
	ch = ready
	for range 3 {
		<-ch
	}
}

func TestDeadlineAssignedAfterLoop(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := (<-chan time.Time)(ready)
	for range 3 {
		<-ch
	}
	ch = time.After(time.Millisecond)
	<-ch
}

func TestDeadlineReassignedBeforeReceive(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Millisecond)
	for range 3 {
		ch = ready
		<-ch
	}
}

func receiveAfterRequiredFor(ready <-chan time.Time) {
	ch := time.After(time.Millisecond)
	for {
		ch = ready
		break
	}
	for range 3 {
		<-ch
	}
	ch = time.After(time.Millisecond)
	for true {
		ch = ready
		break
	}
	for range 3 {
		<-ch
	}
}

func receiveAfterRequiredRange(ready <-chan time.Time) {
	ch := time.After(time.Millisecond)
	for range 1 {
		ch = ready
	}
	for range 3 {
		<-ch
	}
	ch = time.After(time.Millisecond)
	for range "x" {
		ch = ready
	}
	for range 3 {
		<-ch
	}
	ch = time.After(time.Millisecond)
	for range &[1]int{} {
		ch = ready
	}
	for range 3 {
		<-ch
	}
}

func receiveAfterNestedBreaks(ready <-chan time.Time) {
	ch := time.After(time.Millisecond)
	for range 1 {
		switch {
		case testing.Short():
			break
		}
		select {
		default:
			break
		}
		for range 1 {
			break
		}
		ch = ready
	}
	for range 1 {
		<-ch
	}
}

func receiveAfterBreakBeforePost(ready <-chan time.Time) {
	ch := ready
	for ; true; ch = time.After(time.Millisecond) {
		break
	}
	for range 1 {
		<-ch
	}
}

func receiveAfterContinueWithPost(ready <-chan time.Time) {
	ch := ready
	for i := 0; i < 1; ch = ready {
		ch = time.After(time.Millisecond)
		i++
		continue
	}
	for range 1 {
		<-ch
	}
}

func TestDeadlineReassignedOnBothBranches(t *testing.T) {
	ready := make(chan time.Time)
	other := make(chan time.Time)
	close(ready)
	close(other)
	ch := time.After(time.Millisecond)
	if t.Name() == "use readiness channel" {
		ch = ready
	} else {
		ch = other
	}
	for range 3 {
		<-ch
	}
}

func TestDeadlineReassignedByRangeValue(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Millisecond)
	for _, ch = range [1]<-chan time.Time{ready} {
		<-ch
	}
	for range 1 {
		<-ch
	}
}

func TestDeadlineReassignedByRangeKey(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Millisecond)
	for ch = range map[<-chan time.Time]bool{ready: true} {
		<-ch
	}
}

func TestDeadlineReassignedOnEverySwitchCase(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Millisecond)
	switch t.Name() {
	default:
		ch = ready
	case "use different readiness channel":
		other := make(chan time.Time)
		close(other)
		ch = other
	}
	for range 3 {
		<-ch
	}
}

func TestDeadlineReassignedAfterFallthrough(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := (<-chan time.Time)(ready)
	switch t.Name() {
	case "create deadline":
		ch = time.After(time.Millisecond)
		fallthrough
	default:
		ch = ready
	}
	for range 3 {
		<-ch
	}
}

func receiveAfterSelect(replacement <-chan (<-chan time.Time), ready <-chan time.Time) {
	ch := time.After(time.Millisecond)
	select {
	case ch = <-replacement:
	default:
		ch = ready
	}
	for range 3 {
		<-ch
	}
}

func receiveInOtherSelectClause(gate <-chan struct{}, ready <-chan time.Time) {
	ch := ready
	for range 3 {
		select {
		case <-gate:
			ch = time.After(time.Millisecond)
		default:
			<-ch
		}
		ch = ready
	}
}

func TestTimingDoesNotExitLoop(t *testing.T) {
	start := time.Now()
	for range 3 {
		if time.Since(start) > time.Second {
			t.Log("slow")
		}
		if time.Since(start) > time.Minute {
			continue
		}
		if time.Since(start) > time.Second {
			switch t.Name() {
			case "other":
				break
			}
			func() { return }()
		}
	}
}

func TestLoopRecordsUnrelatedSample(t *testing.T) {
	sample := time.Now()
	for i := 0; i < 3; i++ {
		sample = time.Now()
		t.Log(sample)
	}
}

func TestLoopDiscardsSampleBeforeCondition(t *testing.T) {
	ready := true
	for ready {
		ready = time.Now().IsZero()
		t.Log(ready)
		ready = false
	}
}

func TestLoopDiscardsSampleInPost(t *testing.T) {
	ready := true
	for ; ready; ready = false {
		ready = time.Now().IsZero()
		t.Log(ready)
	}
}

func TestLoopShadowsSample(t *testing.T) {
	now := time.Now()
	for i := 0; i < 3 && !now.IsZero(); i++ {
		now := time.Now()
		t.Log(now)
	}
}

func TestStoredTimingOnlyForLog(t *testing.T) {
	start := time.Now()
	for range 3 {
		elapsed := time.Since(start)
		if elapsed > time.Second {
			t.Log("slow")
		}
		if t.Failed() {
			t.Fatalf("failed after %v", elapsed)
		}
	}
}

func TestStoredSampleReplacedBeforeExit(t *testing.T) {
	start := time.Now()
	for range 3 {
		elapsed := time.Since(start)
		t.Log(elapsed)
		elapsed = time.Second
		if elapsed > time.Second {
			t.Fatal("timeout")
		}
	}
}

func TestSnapshotBeforeLoop(t *testing.T) {
	elapsed := time.Since(time.Time{})
	for range 3 {
		if elapsed < time.Second {
			t.Fatal("unexpected initial duration")
		}
	}
}

func TestReceiveAfterReturningBranch(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	if testing.Short() {
		ch = ready
	} else {
		return
	}
	for range 3 {
		<-ch
	}
}

func TestClockSampleOnBreakDoesNotReachNextIteration(t *testing.T) {
	start := time.Now()
	var elapsed time.Duration
	for range 3 {
		if elapsed > time.Second {
			t.Fatal("timeout")
		}
		if testing.Short() {
			elapsed = time.Since(start)
			break
		}
	}
}

func TestRangeOverwritesClockSampleFromPreviousIteration(t *testing.T) {
	start := time.Now()
	var elapsed time.Duration
	for _, elapsed = range [3]time.Duration{} {
		if elapsed > time.Second {
			t.Fatal("timeout")
		}
		elapsed = time.Since(start)
	}
}

func receiveBeforeSettingDeadline(ready <-chan time.Time) {
	ch := ready
	for range 1 {
		<-ch
		ch = time.After(time.Second)
	}
}

func receiveBeforeDeadlineCanReachIt(ready <-chan time.Time) {
	ch, next := ready, ready
	for range 2 {
		<-ch
		ch, next = next, time.After(time.Second)
	}
}

func TestClockSampleAssignedOnOnlyIteration(t *testing.T) {
	start := time.Now()
	var elapsed time.Duration
	for range "é" {
		if elapsed > time.Second {
			t.Fatal("timeout")
		}
		elapsed = time.Since(start)
	}
}

func TestSwitchExitBeforeClockCase(t *testing.T) {
	start := time.Now()
	for range 3 {
		switch {
		case testing.Short():
			t.Skip("short mode")
		case time.Since(start) < time.Second:
		}
	}
}

func TestClockSwitchWithLocalBreak(t *testing.T) {
	start := time.Now()
	for {
		switch {
		case time.Since(start) > time.Second:
			t.Log("slow")
			break
		}
	}
}

func TestClockContinueSkipsWork(t *testing.T) {
	start := time.Now()
	for range 3 {
		if time.Since(start) < time.Second {
			continue
		}
		t.Log("slow")
	}
}

func TestClockContinueInSeparateBranch(t *testing.T) {
	start := time.Now()
	for range 3 {
		if testing.Short() {
			if time.Since(start) < time.Second {
				continue
			}
		} else {
			t.Fatal("unsupported")
		}
	}
}

func TestClockContinueTargetsInnerLoop(t *testing.T) {
	start := time.Now()
	for range 3 {
		if time.Since(start) < time.Second {
			for range 2 {
				continue
			}
		}
		t.Fatal("unsupported")
	}
}

func TestReceiveAfterFatalBranch(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	if testing.Short() {
		ch = ready
	} else {
		t.Fatal("unsupported")
	}
	for range 3 {
		<-ch
	}
}

func TestReceiveAfterSkippedBranch(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	if testing.Short() {
		ch = ready
	} else {
		t.SkipNow()
	}
	for range 3 {
		<-ch
	}
}

func TestImmediateCallReplacesDeadline(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	(func() { ch = ready })()
	for range 3 {
		<-ch
	}
}

func TestImmediateCallReplacesDeadlineBeforeReturn(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	result := func() bool {
		if testing.Short() {
			ch = ready
			return true
		}
		ch = ready
		return false
	}()
	for range 3 {
		<-ch
	}
	t.Log(result)
}

func TestImmediateCallEvaluatesArgumentsFirst(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	func(bool) {
		for range 3 {
			<-ch
		}
	}(func() bool {
		ch = ready
		return true
	}())
}

func TestImmediateCallOnlyReceivesReadinessArgument(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	for range 3 {
		func(first, _ <-chan time.Time) { <-first }(ready, time.After(time.Millisecond))
	}
}

func TestImmediateCallReplacesDeadlineArgument(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	for range 3 {
		func(ch <-chan time.Time) {
			ch = ready
			<-ch
		}(time.After(time.Millisecond))
	}
}

func TestImmediateCallDoesNotRunWhenArgumentEndsTest(t *testing.T) {
	for range 3 {
		func(ch <-chan time.Time, _ bool) { <-ch }(time.After(time.Millisecond), func() bool {
			t.SkipNow()
			return false
		}())
	}
}

func TestImmediateCallDoesNotTreatTupleAsOneClockValue(t *testing.T) {
	values := func(now time.Time) (bool, time.Time) { return testing.Short(), now }
	for range 3 {
		func(skip bool, _ ...time.Time) {
			if skip {
				t.SkipNow()
			}
		}(values(time.Now()))
	}
}

func TestImmediateCallRunsAfterTrueAnd(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	_ = true && func() bool {
		ch = ready
		return true
	}()
	for range 3 {
		<-ch
	}
}

func TestReceiveAfterImmediateSkip(t *testing.T) {
	ch := time.After(time.Second)
	_ = func() bool {
		t.SkipNow()
		return false
	}()
	for range 3 {
		<-ch
	}
}

func TestDelayedCallsDoNotPollEnclosingLoop(t *testing.T) {
	for range 3 {
		defer (func() { <-time.After(time.Millisecond) })()
		go func() { <-time.After(time.Millisecond) }()
		_ = func() { <-time.After(time.Millisecond) }
		func(func()) {}(func() { <-time.After(time.Millisecond) })
	}
}

func TestClockControlledFlagIsReplacedBeforeExit(t *testing.T) {
	start := time.Now()
	for range 3 {
		expired := false
		if time.Since(start) > time.Second {
			expired = true
		}
		expired = testing.Short()
		if expired {
			break
		}
	}
}

func TestUncalledClosureDoesNotSetTimeoutFlag(t *testing.T) {
	start := time.Now()
	for range 3 {
		expired := false
		if time.Since(start) > time.Second {
			_ = func() { expired = true }
		}
		if expired {
			break
		}
	}
}

func TestClockGuardOnlyReturnsFromImmediateCall(t *testing.T) {
	start := time.Now()
	for range 3 {
		func() {
			if time.Since(start) > time.Second {
				return
			}
			t.Log("still within budget")
		}()
	}
}

func TestSwitchExpressionReplacesDeadlineBeforeLaterCasesAndDefault(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	switch {
	default:
		for range 3 {
			<-ch
		}
	case func() bool {
		ch = ready
		return testing.Short()
	}():
	case testing.Verbose():
		for range 3 {
			<-ch
		}
	}
	for range 3 {
		<-ch
	}
}

func TestSwitchExpressionReplacesDeadlineWithoutDefault(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	switch {
	case func() bool {
		ch = ready
		return testing.Short()
	}():
	}
	for range 3 {
		<-ch
	}
}

func TestSelectOperandsReplaceDeadlineBeforeAnyBody(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	sink := make(chan (<-chan time.Time), 1)
	ch := time.After(time.Second)
	var replacement <-chan time.Time
	select {
	default:
		for range 3 {
			<-ch
		}
	case replacement = <-func() <-chan (<-chan time.Time) {
		replacement = ready
		return nil
	}():
	case sink <- func() <-chan time.Time {
		ch = replacement
		return ready
	}():
		for range 3 {
			<-ch
		}
	}
	for range 3 {
		<-ch
	}
}

func TestClockShortCircuitDoesNotTaintFollowingAssignments(t *testing.T) {
	start := time.Now()
	for range 3 {
		expired := false
		_ = time.Since(start) > time.Second && func() bool {
			expired = true
			return true
		}()
		expired = testing.Short()
		if expired {
			break
		}
	}
}

func TestClockOnRightOfShortCircuit(t *testing.T) {
	for {
		_ = func() bool {
			t.Fatal("unconditional termination")
			return false
		}() && time.Now().IsZero()
	}
}

func TestDeferredReceiveAfterDeadlineReplacement(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	defer func() {
		for range 3 {
			<-ch
		}
	}()
	ch = ready
}

func TestReceiveAfterHelperReplacesDeadline(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	replaceDeadline((&(ch)), ready)
	for range 3 {
		<-ch
	}
}

func replaceDeadline(ch *<-chan time.Time, ready <-chan time.Time) { *ch = ready }

func TestReceiveAfterImmediateClosureReplacesDeadline(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := time.After(time.Second)
	func(deadline *<-chan time.Time) {
		*deadline = ready
		for range 3 {
			<-ch
		}
	}(&ch)
}

func TestReceiveAfterImmediateClosureWritesThroughCaptureAndPointer(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	var ch <-chan time.Time
	func(deadline *<-chan time.Time) {
		ch = time.After(time.Second)
		*deadline = ready
	}(&ch)
	for range 3 {
		<-ch
	}
}

func TestReceiveAfterDeferredClosureReplacesReturnedDeadline(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := func() (deadline <-chan time.Time) {
		defer func() { deadline = ready }()
		return time.After(time.Second)
	}()
	for range 3 {
		<-ch
	}
}

func TestReceiveAfterDeferredHelperReplacesReturnedDeadline(t *testing.T) {
	ready := make(chan time.Time)
	close(ready)
	ch := func() (deadline <-chan time.Time) {
		defer replaceDeadline(&deadline, ready)
		return time.After(time.Second)
	}()
	for range 3 {
		<-ch
	}
}
