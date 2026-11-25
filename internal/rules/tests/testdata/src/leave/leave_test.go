package leave

import (
	"testing"
	"time"
)

func TestEnvironment(t *testing.T) {
	t.Setenv("EXAMPLE", "yes")
}

func localCalls() {
	time := struct{ Sleep func() }{Sleep: func() {}}
	time.Sleep()

	os := environment{}
	os.Setenv()
	os.Unsetenv()
}

type environment struct{}

func (environment) Setenv()   {}
func (environment) Unsetenv() {}

var _ = time.Sleep
