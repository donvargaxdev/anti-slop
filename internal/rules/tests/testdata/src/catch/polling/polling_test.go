package polling

import (
	"testing"
	. "time"
)

var packageDeadline = After(Second)

func TestAdjustedElapsedTime(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		elapsed := Since(start)
		elapsed += Millisecond
		if elapsed > Second {
			t.Fatal("timeout")
		}
	}
}

func TestClockOnRightOfCompoundAssignment(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		elapsed := Millisecond
		elapsed += Since(start)
		if elapsed > Second {
			t.Fatal("timeout")
		}
	}
}

func pollPackageDeadline() {
	for range 3 { // want "polling against the wall clock"
		<-packageDeadline
	}
}

func receiveDeadlineWhenReturning() Time {
	for range 1 { // want "polling against the wall clock"
		return <-After(Second)
	}
	return Time{}
}

func TestPoll(t *testing.T) {
	start := Now()
	end := start.Add(Second)
	for Now().Before(end) { // want "polling against the wall clock"
	}
	for Since(start) < Second { // want "polling against the wall clock"
	}
	for Until(end) > 0 { // want "polling against the wall clock"
	}
	for { // want "polling against the wall clock"
		select {
		case <-After(Second):
			return
		default:
		}
	}
}

func pollDeadlineInCondition() {
	for (<-After(Millisecond)).IsZero() { // want "polling against the wall clock"
	}
}

func pollDeadlineInPost(ready func() bool) {
	for ; !ready(); <-After(Millisecond) { // want "polling against the wall clock"
	}
}

func TestPollWithDeadline(t *testing.T) {
	expiry := After(Second)
	var deadline = expiry
	for range 3 { // want "polling against the wall clock"
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
		}
	}
}

func TestClockRefreshedInLoopHeader(t *testing.T) {
	end := Now().Add(Second)
	for now := Now(); now.Before(end); now = Now() { // want "polling against the wall clock"
	}
	start := Now()
	var elapsed Duration
	for i := 0; elapsed < Second && i < 3; i, elapsed = i+1, Since(start) { // want "polling against the wall clock"
	}
}

func TestDeadlineAssignedDuringLoop(t *testing.T) {
	var ch <-chan Time
	for range 3 { // want "polling against the wall clock"
		ch = After(Millisecond)
		<-ch
	}
}

func TestReceiveBeforeParallelReassignment(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := After(Millisecond)
	for range 3 { // want "polling against the wall clock"
		ch, _ = ready, <-ch
	}
}

func TestDeadlineOnOneBranch(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := After(Millisecond)
	if t.Name() == "use readiness channel" {
		ch = ready
	}
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func TestDeadlineOnUnmatchedSwitch(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := After(Millisecond)
	switch t.Name() {
	case "use readiness channel":
		ch = ready
	}
	for { // want "polling against the wall clock"
		select {
		case <-ch:
			return
		default:
		}
	}
}

func TestDeadlineOnOneSwitchCase(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	switch ch := After(Millisecond); t.Name() {
	case "use readiness channel":
		ch = ready
	default:
		for range 3 { // want "polling against the wall clock"
			<-ch
		}
	}
}

func TestDeadlineFallsThrough(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := (<-chan Time)(ready)
	switch t.Name() {
	case "use deadline":
		ch = After(Millisecond)
		if ch != nil {
			goto receive
		}
	receive:
		fallthrough
	default:
		for range 3 { // want "polling against the wall clock"
			<-ch
		}
	}
}

func TestDeadlineOnOneTypeSwitchCase(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := (<-chan Time)(ready)
	switch deadline := After(Millisecond); any(t).(type) {
	case *testing.T:
		ch = deadline
	default:
		ch = ready
	}
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func pollAfterSelect(gate <-chan struct{}, ready <-chan Time) {
	ch := After(Millisecond)
	select {
	case <-gate:
		ch = ready
	default:
	}
	for { // want "polling against the wall clock"
		select {
		case <-ch:
			return
		default:
		}
	}
}

func pollInSelectCommunication(replacement <-chan (<-chan Time)) {
	ch := After(Millisecond)
	for range 1 { // want "polling against the wall clock"
		select {
		case ch = <-replacement:
		case <-ch:
		}
	}
}

func TestDeadlineInSwitchExpressions(t *testing.T) {
	ch := After(Millisecond)
	for range 1 { // want "polling against the wall clock"
		switch <-ch {
		case Time{}:
		}
	}
	ch = After(Millisecond)
	for range 1 { // want "polling against the wall clock"
		switch (Time{}) {
		case <-ch:
		}
	}
}

func TestDeadlineAssignedForNextIteration(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := (<-chan Time)(ready)
	for range 3 { // want "polling against the wall clock"
		<-ch
		ch = After(Millisecond)
	}
}

func pollAfterOptionalRange(n int, ready <-chan Time) {
	ch := After(Millisecond)
	for range n {
		ch = ready
		<-ch
	}
	for { // want "polling against the wall clock"
		select {
		case <-ch:
			return
		default:
		}
	}
}

func pollAfterEarlyRangeBreak(ready <-chan Time) {
	ch := After(Millisecond)
	for range 1 {
		if testing.Short() {
			break
		}
		ch = ready
	}
	for { // want "polling against the wall clock"
		select {
		case <-ch:
			return
		default:
		}
	}
}

func pollAfterLabeledBreak(ready <-chan Time) {
	ch := After(Millisecond)
loop:
	for range 1 {
		select {
		default:
			for range 1 {
				if testing.Short() {
					break loop
				}
			}
		}
		ch = ready
	}
	for range 1 { // want "polling against the wall clock"
		<-ch
	}
}

func pollAfterBreakBeforePost(ready <-chan Time) {
	ch := After(Millisecond)
	for ; true; ch = ready {
		break
	}
	for range 1 { // want "polling against the wall clock"
		<-ch
	}
}

func pollAfterContinue(ready <-chan Time) {
	ch := After(Millisecond)
	for range 1 {
		switch testing.Short() {
		case true:
			continue
		}
		ch = ready
	}
	for range 1 { // want "polling against the wall clock"
		<-ch
	}
}

func pollAfterLabeledContinue(ready <-chan Time) {
	ch := After(Millisecond)
outer:
	for range 1 {
		for range 1 {
			select {
			default:
				if testing.Short() {
					continue outer
				}
			}
		}
		ch = ready
	}
	for range 1 { // want "polling against the wall clock"
		<-ch
	}
}

func pollAfterOptionalFor(run bool, ready <-chan Time) {
	ch := After(Millisecond)
	for run {
		ch = ready
		break
	}
	for { // want "polling against the wall clock"
		<-ch
		return
	}
}

func pollAfterEmptyLoops(ready <-chan Time) {
	ch := After(Millisecond)
	for false {
		ch = ready
	}
	for range 0 {
		ch = ready
	}
	for range "" {
		ch = ready
	}
	for range [0]int{} {
		ch = ready
	}
	for range 1 { // want "polling against the wall clock"
		<-ch
	}
}

func TestPollUntilBreak(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		if Since(start) > Second {
			break
		}
	}
}

func TestPollUntilReturn(t *testing.T) {
	end := Now().Add(Second)
	for range 3 { // want "polling against the wall clock"
		if Now().Before(end) {
			continue
		} else {
			return
		}
	}
}

func TestPollUntilLabeledBreak(t *testing.T) {
	end := Now().Add(Second)
poll:
	for { // want "polling against the wall clock"
		select {
		default:
			if Until(end) <= 0 {
				break poll
			}
		}
	}
}

func TestPollUntilFatal(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		if Since(start) > Second {
			t.Fatal("timeout")
		}
	}
}

func TestPollWithLabeledContinueBeforeExit(t *testing.T) {
	start := Now()
poll:
	for { // want "polling against the wall clock"
		if elapsed := Since(start); elapsed < Second {
			for range 2 {
				continue poll
			}
		}
		break
	}
}

func pollWithClockSwitchBeforeReturn(start Time) {
	for { // want "polling against the wall clock"
		switch {
		case Since(start) < Second:
			continue
		}
		return
	}
}

func pollWithContinueInDefault(start Time) {
	for { // want "polling against the wall clock"
		switch {
		default:
			continue
		case Since(start) >= Second:
		}
		return
	}
}

func TestPollUntilFatalf(t *testing.T) {
	start := Now()
	for range 3 { // want "polling against the wall clock"
		if elapsed := Since(start); elapsed > Second {
			t.Fatalf("timed out after %v", elapsed)
		}
	}
}

func TestPollWithAssignedClock(t *testing.T) {
	start := Now()
	var remaining Duration
	for { // want "polling against the wall clock"
		if remaining = Until(start.Add(Second)); remaining <= 0 {
			break
		}
	}
}

func TestPollUntilFailNow(t *testing.T) {
	deadline := Now().Add(Second)
	for { // want "polling against the wall clock"
		if Until(deadline) <= 0 {
			t.FailNow()
		}
	}
}

func TestClockRefreshedInLoopBody(t *testing.T) {
	now := Now()
	end := now.Add(Second)
	for now.Before(end) { // want "polling against the wall clock"
		now = Now()
	}
}

func TestClockRefreshedThroughTemporary(t *testing.T) {
	start := Now()
	var elapsed Duration
	for elapsed < Second { // want "polling against the wall clock"
		sample := Since(start)
		elapsed = sample
	}
}

func TestClockRefreshedBeforeContinue(t *testing.T) {
	start := Now()
	var elapsed Duration
poll:
	for elapsed < Second { // want "polling against the wall clock"
		if testing.Short() {
			elapsed = Since(start)
			continue poll
		}
		elapsed = Second
	}
}

func TestPollWithStoredSample(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		elapsed := Since(start)
		if elapsed > Second {
			t.Fatal("timeout")
		}
	}
}

func TestPollWithConditionAssignedInBranch(t *testing.T) {
	start := Now()
	for range 3 { // want "polling against the wall clock"
		expired := false
		if testing.Short() {
			elapsed := Since(start)
			expired = elapsed > Second
		}
		if expired {
			return
		}
	}
}

func TestPollWithSampleFromPreviousIteration(t *testing.T) {
	start := Now()
	var elapsed Duration
	for { // want "polling against the wall clock"
		if elapsed > Second {
			t.Fatal("timeout")
		}
		elapsed = Since(start)
	}
}

func TestPollWithSampleFromContinueAndPost(t *testing.T) {
	start := Now()
	var elapsed, sample Duration
	for ; ; elapsed = sample { // want "polling against the wall clock"
		if elapsed > Second {
			t.Fatal("timeout")
		}
		if testing.Short() {
			sample = Since(start)
			continue
		}
		sample = 0
	}
}

func TestPollWithSamplePassedAcrossIterations(t *testing.T) {
	start := Now()
	var elapsed, sample Duration
	for range 3 { // want "polling against the wall clock"
		if elapsed > Second {
			t.Fatal("timeout")
		}
		elapsed, sample = sample, Since(start)
	}
}

func TestPollUntilSwitchFatal(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		switch {
		case Since(start) > Second:
			t.Fatal("timeout")
		default:
		}
	}
}

func TestPollUntilSwitchLabeledBreak(t *testing.T) {
	start := Now()
poll:
	for { // want "polling against the wall clock"
		switch elapsed := Since(start); elapsed > Second {
		case true:
			break poll
		}
	}
}

func TestPollUntilSwitchDefault(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		switch {
		default:
			return
		case Since(start) <= Second:
		}
	}
}

func TestPollUntilSwitchFallthrough(t *testing.T) {
	start := Now()
	for { // want "polling against the wall clock"
		switch {
		case Since(start) > Second:
			fallthrough
		case testing.Short():
			t.FailNow()
		}
	}
}

func TestReceiveAfterNonTerminatingCall(t *testing.T) {
	ch := After(Second)
	t.Error("nonfatal")
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func TestReceiveAfterDeferredFatal(t *testing.T) {
	ch := After(Second)
	defer t.Fatal("deferred")
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func TestDeadlineInFatalArgument(t *testing.T) {
	for range 3 { // want "polling against the wall clock"
		t.Fatal(<-After(Second))
	}
}

func TestClockGuardEndingWithSkip(t *testing.T) {
	started := Now()
	for { // want "polling against the wall clock"
		if Since(started) > Second {
			t.Skip("timeout")
		}
	}
}

func TestImmediateCallInstallsDeadline(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	var ch <-chan Time = ready
	_ = testing.Short() && func() bool {
		ch = After(Second)
		return true
	}()
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func TestDeadlineInImmediateCall(t *testing.T) {
	deadline := After(Millisecond)
	for range 3 { // want "polling against the wall clock"
		(func() {
			func() { <-deadline }()
		})()
	}
}

func TestDeadlineArgumentInImmediateCall(t *testing.T) {
	for range 3 { // want "polling against the wall clock"
		func(ch <-chan Time) { <-ch }(After(Millisecond))
	}
}

func TestDeadlineArgumentInNestedImmediateCall(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	for range 3 { // want "polling against the wall clock"
		(func(first, second <-chan Time, _ ...bool) {
			func(ch <-chan Time) { <-ch }(second)
		})(ready, After(Millisecond), true)
	}
}

func TestImmediateCallCapturesArgumentBeforeLaterArgumentRuns(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	for range 3 { // want "polling against the wall clock"
		deadline := After(Millisecond)
		func(ch <-chan Time, _ bool) { <-ch }(deadline, func() bool {
			deadline = ready
			return true
		}())
	}
}

func TestDeadlineInImmediateCondition(t *testing.T) {
	for func() bool { return (<-After(Millisecond)).IsZero() }() { // want "polling against the wall clock"
	}
}

func TestDeadlineInImmediatePost(t *testing.T) {
	for i := 0; i < 3; func() { <-After(Millisecond); i++ }() { // want "polling against the wall clock"
	}
}

func TestClockGuardInsideImmediateCall(t *testing.T) {
	start := Now()
	for range 3 { // want "polling against the wall clock"
		func() {
			if Since(start) > Second {
				t.Fatalf("timeout")
			}
		}()
	}
}

func TestClockGuardCallsImmediateTestExit(t *testing.T) {
	start := Now()
	for range 3 { // want "polling against the wall clock"
		if Since(start) > Second {
			func() { t.SkipNow() }()
		}
	}
}

func TestDeadlineInDeferredCallArgument(t *testing.T) {
	for range 3 { // want "polling against the wall clock"
		defer t.Log(func() Time { return <-After(Millisecond) }())
	}
}

func TestDeferredCallDoesNotReplaceDeadline(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := After(Second)
	defer func() { ch = ready }()
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}

func TestShortCircuitedCallDoesNotReplaceDeadline(t *testing.T) {
	ready := make(chan Time)
	close(ready)
	ch := After(Second)
	_ = true || func() bool {
		ch = ready
		return false
	}()
	_ = testing.Short() && func() bool {
		ch = ready
		return true
	}()
	for range 3 { // want "polling against the wall clock"
		<-ch
	}
}
