package flakes

import (
	"os"
	"strconv"
	"testing"
)

type fakeTest struct{}

func (fakeTest) Skip(string) {}

func TestCases(t *testing.T) {
	if testing.Short() {
		t.Skip("long test")
	}
	if _, ok := os.LookupEnv("MODE"); !ok {
		t.Skip("MODE is missing")
	}
	for _, value := range []string{"a", "b"} {
		t.Run(value, func(t *testing.T) {
			if value == "" {
				t.Error("empty value")
			}
		})
	}
	for i := 0; i < 3; i++ {
		t.Log(i)
	}
	cases := []string{"a", "b"}
	for i := 0; i < len(cases); i++ {
		t.Run(cases[i], func(t *testing.T) {})
	}
	for i := range len(cases) {
		t.Run(cases[i], func(t *testing.T) {})
	}
}

func TestNumericCases(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {})
	}
	count := 3
	for i := range count {
		passed := t.Run(strconv.Itoa(i), func(t *testing.T) {})
		t.Log(i, passed)
	}
}

func check() error { return nil }

func TestFailure(t *testing.T) {
	if err := check(); err != nil {
		t.Fatal(err)
		fakeTest{}.Skip("not a testing method")
	}
}
