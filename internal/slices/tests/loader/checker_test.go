package loader_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
	"github.com/donvargaxdev/anti-slop/internal/slices/tests/loader"
)

func TestCheckerContract(t *testing.T) {
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: loader.Checker{Dir: filepath.Join("..", "..", "..", "..")},
			Clean:   "./internal/rules/tests/testdata/src/leave/sleep",
			Dirty:   "./internal/rules/tests/testdata/src/catch/sleep",
			Broken:  "./internal/rules/tests/testdata/src/missing/...",
		}
	})
}

func TestCheckerReportsLoadFailures(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	emptyModule := t.TempDir()
	if err := os.WriteFile(filepath.Join(emptyModule, "go.mod"), []byte("module example.test/empty\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		dir        string
		pattern    string
		wantReason string
	}{
		{"missing working directory", filepath.Join(emptyModule, "missing"), ".", ""},
		{"no matching package", emptyModule, "./...", "the patterns match no package"},
		{"invalid package", root, "./internal/rules/tests/testdata/src/missing", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found, failure := (loader.Checker{Dir: tc.dir}).Check(t.Context(), []string{tc.pattern})
			var failed *port.Failed
			if !errors.As(failure, &failed) || failed.Reason == "" || len(found) != 0 {
				t.Fatalf("Check(%q) = %v, %v, want no findings and a load failure with a reason", tc.pattern, found, failure)
			}
			if tc.wantReason != "" && failed.Reason != tc.wantReason {
				t.Errorf("Check(%q) reason = %q, want %q", tc.pattern, failed.Reason, tc.wantReason)
			}
		})
	}
}
