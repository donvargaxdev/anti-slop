package conditions

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

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
