package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
)

const recursiveSource = `package catch

func direct() { direct(); direct() }
func first() { second() }
func second() { first() }
func a() { b() }
func b() { c() }
func generic[T any](v T) { (generic[T])(v) }
func pair[A, B any](a A, b B) { pair[A, B](a, b) }
type node[T any] struct{}
func (n *node[T]) walk() { (*node[T]).walk(n) }
type edge struct{}
func (e edge) follow() { e.back() }
func (e edge) back() { e.follow() }
`

const cleanSource = `package leave

import "strings"

func a() { b() }
func b() { c() }
func c() { d() }
func d() { a() }
func leaf() { _ = strings.TrimSpace(""); _ = len(""); _ = int(2) }
func first() { leaf() }
func indirect(f func(), callbacks []func()) { f(); callbacks[0]() }
func shadowed() { shadowed := func() {}; shadowed() }
func callback() func() { return func() { callback() } }
type caller interface{ call() }
type receiver struct{ next caller }
type other struct{}
func (r receiver) call() { r.next.call() }
func (other) call() {}
func forward(r other) { r.call() }
`

func TestReportsRecursion(t *testing.T) {
	recursionModule(t)
	writeRecursionFile(t, "catch/catch.go", recursiveSource)
	writeRecursionFile(t, "catch/other.go", "package catch\n\nfunc c() { a() }\n")
	writeRecursionFile(t, "leave/leave.go", cleanSource)
	want := filepath.FromSlash(`catch/catch.go:3:6: recursion: direct calls itself
catch/catch.go:4:6: recursion: first calls itself through a cycle
catch/catch.go:5:6: recursion: second calls itself through a cycle
catch/catch.go:6:6: recursion: a calls itself through a cycle
catch/catch.go:7:6: recursion: b calls itself through a cycle
catch/catch.go:8:6: recursion: generic calls itself
catch/catch.go:9:6: recursion: pair calls itself
catch/catch.go:11:19: recursion: walk calls itself
catch/catch.go:13:15: recursion: follow calls itself through a cycle
catch/catch.go:14:15: recursion: back calls itself through a cycle
catch/other.go:3:6: recursion: c calls itself through a cycle
`)
	code, stdout, stderr := runRecursion(t, diskAdapters(), "./...")
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("recursion ./... = exit %d, stdout %q, stderr %q; want exit 0, stdout %q, and empty stderr", code, stdout, stderr, want)
	}
}

func TestCheckerContract(t *testing.T) {
	t.Run("tests", func(t *testing.T) {
		t.Chdir("../../internal/rules/tests/testdata/src")
		checkerContract(t, diskAdapters().Tests)
	})
	t.Run("recursion", func(t *testing.T) {
		recursionModule(t)
		writeRecursionFile(t, "catch/catch.go", "package catch\nfunc first() { second() }\nfunc second() { first() }\n")
		writeRecursionFile(t, "leave/leave.go", "package leave\nfunc leaf() {}\n")
		field := reflect.ValueOf(diskAdapters()).FieldByName("Recursion")
		if !field.IsValid() {
			t.Fatal("disk adapters must provide the recursion checker")
		}
		checkerContract(t, field.Interface().(port.Checker))
	})
}

func checkerContract(t *testing.T, checker port.Checker) {
	t.Helper()
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: checker,
			Clean:   "./leave",
			Dirty:   "./catch",
			Broken:  "./missing",
		}
	})
}

func TestRecursionReportsPhysicalSourcePosition(t *testing.T) {
	recursionModule(t)
	writeRecursionFile(t, "p.go", "package p\n\n//line virtual.go:100:20\nfunc direct() { direct() }\n")
	want := "p.go:4:6: recursion: direct calls itself\n"
	code, stdout, stderr := runRecursion(t, diskAdapters(), "./...")
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("recursion ./... = exit %d, stdout %q, stderr %q; want exit 0, stdout %q, and empty stderr", code, stdout, stderr, want)
	}
}

func TestCheckerDiscardsFindingsOnAnalyzerFailure(t *testing.T) {
	recursionModule(t)
	writeRecursionFile(t, "p.go", "package p\n")
	rule := &analysis.Analyzer{
		Run: func(pass *analysis.Pass) (any, error) {
			pass.Reportf(pass.Files[0].Pos(), "partial finding")
			return nil, errors.New("analysis failed")
		},
	}
	provider, ok := diskAdapters().Tests.(interface {
		WithAnalyzer(*analysis.Analyzer) port.Checker
	})
	if !ok {
		t.Fatal("the typed checker must accept a Go analyzer")
	}
	found, failure := provider.WithAnalyzer(rule).Check(t.Context(), []string{"."})
	failed, ok := failure.(*port.Failed)
	if found != nil || !ok || failed.Reason != "analysis failed" {
		t.Errorf("Check returned %v, %v; want no findings and an analysis failure", found, failure)
	}
}

func TestCheckerRejectsAnEmptyPackageMatch(t *testing.T) {
	recursionModule(t)
	code, stdout, stderr := runRecursion(t, diskAdapters(), "./...")
	if code != cli.CodeEnvironment || stdout != "" || !strings.Contains(stderr, "the patterns match no package") {
		t.Errorf("recursion ./... = exit %d, stdout %q, stderr %q; want a no-match failure", code, stdout, stderr)
	}
}

func TestCheckerKeepsAbsolutePathForFindingsOutsideItsDirectory(t *testing.T) {
	dir := recursionModule(t)
	writeRecursionFile(t, "catch/catch.go", "package catch\nfunc direct() { direct() }\n")
	writeRecursionFile(t, "leave/leave.go", "package leave\n")
	t.Chdir("leave")
	path := filepath.Join(dir, "catch", "catch.go")
	want := path + ":2:6: recursion: direct calls itself\n"
	code, stdout, stderr := runRecursion(t, diskAdapters(), path)
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("recursion %q = exit %d, stdout %q, stderr %q; want exit 0, stdout %q, and empty stderr", path, code, stdout, stderr, want)
	}
}

type recursionFailure struct{ *port.Failed }

func TestClassifyCheckerFailures(t *testing.T) {
	classify := recursionClassifier(t)
	for _, tc := range []struct {
		failure error
		code    int
		rule    string
	}{
		{&port.Failed{Reason: "invalid source"}, cli.CodeEnvironment, "recursion.check-failed"},
		{&port.Cancelled{}, cli.CodeInterrupted, "interrupted"},
		{recursionFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal, "internal"},
	} {
		problems, ok := classify(tc.failure)
		if !ok || len(problems) != 1 || problems[0].Code != tc.code || problems[0].Rule != tc.rule {
			t.Errorf("Classify(%T) = %v, %v; want one problem with code %d and rule %q", tc.failure, problems, ok, tc.code, tc.rule)
		}
	}
}

func TestClassifyDeclinesUnrelatedErrors(t *testing.T) {
	classify := recursionClassifier(t)
	if problems, ok := classify(errors.New("unrelated")); ok || problems != nil {
		t.Errorf("Classify(unrelated) = %v, %v; want nil, false", problems, ok)
	}
}

func recursionClassifier(t *testing.T) cli.Classifier {
	t.Helper()
	field := reflect.ValueOf(commandLine{}).FieldByName("Recursion")
	if !field.IsValid() {
		t.Fatal("the recursion command must register a classifier")
	}
	return field.Interface().(interface {
		Classify(error) ([]*cli.Problem, bool)
	}).Classify
}

func recursionModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	writeRecursionFile(t, "go.mod", "module example.test/recursion\n\ngo 1.27\n")
	return dir
}

func writeRecursionFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runRecursion(t *testing.T, a adapters, patterns ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), append([]string{"recursion"}, patterns...), strings.NewReader(""), &stdout, &stderr, a)
	return code, stdout.String(), stderr.String()
}
