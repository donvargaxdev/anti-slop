package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchCaseTables(t *testing.T) {
	table := "[]int{\n" + strings.Repeat("0,\n", 71) + "}"
	const runCases = "\nfor range cases {}\n"
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"either arm supplies cases", `
var cases []int
if a { cases = TABLE } else { cases = TABLE }
` + runCases, 0},
		{"optional assignment preserves earlier cases", `
cases := TABLE
if a { cases = TABLE }
` + runCases, 0},
		{"else if supplies another table", `
var cases []int
if a { cases = TABLE } else if b { cases = TABLE } else { cases = TABLE }
` + runCases, 0},
		{"both arms overwrite the earlier table", `
cases := TABLE
if a { cases = TABLE } else { cases = TABLE }
` + runCases, 1},
		{"later assignment overwrites both arms", `
var cases []int
if a { cases = TABLE } else { cases = TABLE }
cases = nil
` + runCases, 1},
		{"range in else cannot use then assignment", `
var cases []int
if a { cases = TABLE } else { for range cases {} }
`, 1},
		{"branch declaration cannot replace outer table", `
cases := TABLE
if a { cases := TABLE; _ = cases }
` + runCases, 1},
		{"initializer table is visible in both arms", `
if cases := TABLE; a { for range cases {} } else { for range cases {} }
`, 0},
		{"earlier assignment in an arm is overwritten", `
var cases []int
if a { cases = TABLE; cases = TABLE } else { cases = TABLE }
` + runCases, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const header = "package example\nfunc test(a, b bool) {\n"
			source := header + strings.ReplaceAll(tc.body, "TABLE", table) + "}\n"
			assertBranchDiagnostics(t, source, tc.want)
		})
	}
}

func assertBranchDiagnostics(t *testing.T, source string, want int) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/m\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cases_test.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"funclen", "./..."}, strings.NewReader(""), &stdout, &stderr, diskAdapters())
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("funclen: want exit 0 and empty stderr, got %d, stderr %q", code, stderr.String())
	}
	if got := strings.Count(stdout.String(), ": funclen: "); got != want {
		t.Errorf("got %d diagnostics, want %d: %q", got, want, stdout.String())
	}
}
