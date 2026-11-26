package conditions

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Drive the fixtures through the command so the same test can run before the
// conditions analyzer and its command exist.
func TestCompoundConditions(t *testing.T) {
	const reason = ": conditions: split this condition into named booleans\n"
	path := filepath.Join("testdata", "src", "catch", "catch.go")
	want := path + ":4:5" + reason +
		path + ":5:12" + reason +
		path + ":7:6" + reason +
		path + ":10:14" + reason +
		path + ":12:5" + reason +
		path + ":15:6" + reason

	for _, tc := range []struct {
		fixture string
		want    string
	}{
		{"catch", want},
		{"leave", ""},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "go", "run", "-buildvcs=false",
				"../../../cmd/anti-slop", "conditions", "./testdata/src/"+tc.fixture+"/...")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil || stderr.Len() != 0 || string(out) != tc.want {
				t.Errorf("conditions %s returned %v, stdout %q, stderr %q; want success, stdout %q and no stderr",
					tc.fixture, err, out, stderr.String(), tc.want)
			}
		})
	}
}

func TestConditionsReportsPackagesThatCannotBeLoaded(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "run", "-buildvcs=false",
		"../../../cmd/anti-slop", "conditions", "./testdata/missing/...")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil || len(out) != 0 || !strings.Contains(stderr.String(), "anti-slop: the packages cannot be loaded") {
		t.Errorf("conditions returned %v, stdout %q, stderr %q; want a package loading error", err, out, stderr.String())
	}
}
