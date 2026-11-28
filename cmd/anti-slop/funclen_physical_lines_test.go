package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteriorLines(t *testing.T) {
	const header = "package example\nfunc cases() {\n"
	const table = "for range []int{\n1,\n\n2,\n} {\n}\n}\n"
	const want = "cases_test.go:2:1: funclen: function body is 71 lines; limit is 70\n"
	if !t.Run("71 counted lines", func(t *testing.T) {
		assertFunctionLengthOutput(t, "cases_test.go", header+strings.Repeat("_ = 0\n", 67)+table, want)
	}) {
		return
	}
	t.Run("70 counted lines", func(t *testing.T) {
		assertFunctionLengthOutput(t, "cases_test.go", header+strings.Repeat("_ = 0\n", 66)+table, "")
	})
	t.Run("empty body", assertEmptyBodyLineCount)
}

// The reporting threshold hides a zero-to-one miscount. Overlay a same-package
// test for the exact count after the CLI assertion has established the rule exists.
func assertEmptyBodyLineCount(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const name = "interior_lines_test.go"
	rule := filepath.Join(root, "internal", "rules", "funclen")
	dir := t.TempDir()
	probe := filepath.Join(dir, name)
	if err := os.WriteFile(probe, []byte(emptyBodyProbe), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: map[string]string{filepath.Join(rule, name): probe}})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(file, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "test", "-overlay", file, "-run", "^TestEmptyBodyLineCount$", ".")
	cmd.Dir = rule
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("empty body line count: %v\n%s", err, output)
	}
}

const emptyBodyProbe = `package funclen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEmptyBodyLineCount(t *testing.T) {
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "body.go", "func() {}", parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	body := expr.(*ast.FuncLit).Body
	if got := interiorLines(fset, body.Lbrace, body.Rbrace); got != 0 {
		t.Errorf("empty body has %d interior lines, want 0", got)
	}
}
`

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

func TestProductionFunctionCountsCaseData(t *testing.T) {
	source := "package example\nfunc cases() {\nfor range []int{\n" +
		strings.Repeat("0,\n", 68) + "} {\n}\n}\n"
	const want = "cases.go:2:1: funclen: function body is 71 lines; limit is 70\n"
	assertFunctionLengthOutput(t, "cases.go", source, want)
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
