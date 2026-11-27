package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionBodyUsesPhysicalLines(t *testing.T) {
	source := "package example\n//line body.go:1000\nfunc long() {\n" +
		strings.Repeat("_ = 0\n", 70) + "//line body.go:1\n}\n"
	const want = "body.go:3:1: funclen: function body is 71 lines; limit is 70\n"
	assertFunctionLengthOutput(t, "body.go", source, want)
}

func TestCaseTableUsesPhysicalLines(t *testing.T) {
	source := "package example\nfunc testCases() {\nfor range\n//line cases_test.go:1000\n[]int{\n" +
		strings.Repeat("0,\n", 71) + "//line cases_test.go:1\n} {\n}\n}\n"
	assertFunctionLengthOutput(t, "cases_test.go", source, "")
}

func assertFunctionLengthOutput(t *testing.T, filename, source, want string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/m\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"funclen", "./..."}, strings.NewReader(""), &stdout, &stderr, diskAdapters())
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Errorf("funclen returned exit %d, stdout %q, stderr %q; want exit 0, stdout %q, and empty stderr",
			code, stdout.String(), stderr.String(), want)
	}
}
