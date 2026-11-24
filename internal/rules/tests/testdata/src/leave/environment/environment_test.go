package environment

import (
	"os"
	"testing"
)

type fakeOS struct{}

func (fakeOS) Setenv(string, string) {}

func TestEnv(t *testing.T) {
	t.Setenv("MODE", "test")
	t.Log(os.Getenv("MODE"))
	os := fakeOS{}
	os.Setenv("MODE", "test")
}
