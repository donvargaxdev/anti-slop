package environment

import (
	process "os"
	"testing"
)

func TestEnv(t *testing.T) {
	_ = process.Setenv("MODE", "test") // want "os.Setenv changes the process environment in test code"
	_ = process.Unsetenv("MODE")       // want "os.Unsetenv changes the process environment in test code"
}
