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
