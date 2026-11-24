package sleep_test

import "testing"

type fakeClock struct{}

func (fakeClock) Sleep(int) {}

func TestWait(t *testing.T) {
	time := fakeClock{}
	time.Sleep(5)
}
