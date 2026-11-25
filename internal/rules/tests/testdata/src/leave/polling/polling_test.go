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

func TestClockSwitchWithLocalBreak(t *testing.T) {
	start := time.Now()
	for {
		switch {
		case time.Since(start) > time.Second:
			t.Log("slow")
			break
		case testing.Short():
			return
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
