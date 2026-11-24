package polling

import "time"

func Wait() {
	for time.Since(time.Now()) < time.Second {
	}
}
