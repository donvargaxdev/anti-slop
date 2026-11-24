package sleep

import (
	"testing"
	clock "time"
)

func helper() { clock.Sleep(clock.Second) } // want "time.Sleep in test code"

func TestWait(t *testing.T) {
	helper()
	clock.Sleep(clock.Millisecond) // want "time.Sleep in test code"
}
