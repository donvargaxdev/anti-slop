package catch

import (
	env "os"
	clock "time"
)

func change() {
	clock.Sleep(clock.Second)        // want "avoid time.Sleep in tests"
	_ = env.Setenv("EXAMPLE", "yes") // want "use t.Setenv instead of os.Setenv"
	_ = env.Unsetenv("EXAMPLE")      // want "avoid os.Unsetenv in tests"
}
