package polling

import (
	"testing"
	"time"
)

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
