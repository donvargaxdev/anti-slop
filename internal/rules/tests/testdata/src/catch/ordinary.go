package catch

import (
	"os"
	"time"
)

func ordinaryCode() {
	time.Sleep(time.Second)
	_ = os.Setenv("EXAMPLE", "yes")
	_ = os.Unsetenv("EXAMPLE")
}
