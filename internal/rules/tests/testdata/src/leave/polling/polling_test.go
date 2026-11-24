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
