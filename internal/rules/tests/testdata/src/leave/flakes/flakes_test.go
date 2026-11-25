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
		if !t.Run(cases[i], func(t *testing.T) {}) {
			t.Log("case failed")
		}
	}
	for _, name := range cases {
		if !t.Run(name, func(t *testing.T) {}) {
			t.Log("case failed")
		}
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

func TestContinueToNextCase(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := validate(i); err != nil {
			t.Error(err)
			continue
		}
		t.Run(strconv.Itoa(i), func(t *testing.T) {})
	}
}

func validate(i int) error {
	if i == 1 {
		return os.ErrInvalid
	}
	return nil
}

func check() error { return nil }

func TestFailure(t *testing.T) {
	if err := check(); err != nil {
		t.Fatal(err)
		fakeTest{}.Skip("not a testing method")
	}
}

func TestSkipOnShortOrFailure(t *testing.T) {
	if t.Failed() || testing.Short() {
		t.Skip("optional test")
	}
}

func TestIndependentChecks(t *testing.T) {
	err := check()
	other := err
	for i := 0; i < 3; i++ {
		other = check()
		if err == nil {
			break
		}
		if i > 0 {
			t.Log(other)
		}
		if testing.Short() {
			break
		}
	}

	state := struct{ err error }{err}
	for range 3 {
		if state.err == nil {
			break
		}
	}
}

func TestChecksWithoutRetryExits(t *testing.T) {
	for {
		if t.Name() != "" {
			break
		}
	}
	for range 3 {
		if available() {
			continue
		}
		if _, err := os.Stat("fixture"); err == nil {
			t.Log("fixture exists")
		}
		if check() != nil {
			// Check every iteration even when it fails.
		}
	}
}

func TestContinueWithConditionalSuccessExit(t *testing.T) {
	for i := range 3 {
		if err := validate(i); err != nil {
			t.Error(err)
			continue
		}
		t.Log("case passed")
		if testing.Short() {
			break
		}
	}
}

func TestSkipOnIndependentBranch(t *testing.T) {
	skip := false
	if testing.Short() {
		skip = t.Failed()
	} else if skip {
		t.Skip("optional test")
	}
}

func TestSkipOnOverwrittenOutcome(t *testing.T) {
	skip := t.Failed()
	if testing.Short() {
		skip = available()
	} else {
		skip = false
	}
	if skip {
		t.Skip("optional test")
	}
}

func TestSkipBeforeCleanupAssignsFailure(t *testing.T) {
	skip := testing.Short()
	t.Cleanup(func() { skip = t.Failed() })
	if skip {
		t.Skip("optional test")
	}
}

func TestStopOnStoredFailure(t *testing.T) {
	outcome := t.Run("setup", func(t *testing.T) {})
	for i := range 3 {
		if outcome {
			t.Skip("setup passed")
		}
		outcome = !t.Run(strconv.Itoa(i), func(t *testing.T) {})
		if outcome {
			break
		}
	}
}

func available() bool { return true }

func retry(body any) {}

func TestRetryNonTestCallback(t *testing.T) {
	retry(func(value *int) { *value = 1 })
	retry(func(value int) int { return value + 1 })
}
