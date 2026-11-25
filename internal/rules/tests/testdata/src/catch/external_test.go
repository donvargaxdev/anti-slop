package catch_test

import . "time"

func wait() {
	(Sleep)(Second) // want "avoid time.Sleep in tests"
}
