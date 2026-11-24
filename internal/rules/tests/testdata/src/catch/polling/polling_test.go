package polling

import (
	"testing"
	. "time"
)

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
