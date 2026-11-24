package polling

import (
	"testing"
	"time"
)

type fakeClock struct{}

func (fakeClock) Now() bool { return false }

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
