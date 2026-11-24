package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/slices/comments/loader"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port/porttest"
)

func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.test/m\n\ngo 1.27\n"
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTheLoaderHoldsTheContract(t *testing.T) {
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		dir := module(t, map[string]string{
			"clean/clean.go":   "package clean\n\n//go:noinline\nfunc F() {}\n",
			"dirty/dirty.go":   "package dirty\n\n// F does a thing.\nfunc F() {}\n\n// G does another.\nfunc G() {}\n",
			"broken/broken.go": "package broken\n\nfunc F( {\n",
		})
		return porttest.CheckerCase{Checker: loader.Checker{Dir: dir}, Clean: "./clean", Dirty: "./dirty", Broken: "./broken"}
	})
}

func TestTheLoaderGivesPathsBelowItsFolderAndTheirPlace(t *testing.T) {
	dir := module(t, map[string]string{"p/p.go": "package p\n\nfunc F() {\n\tx := 1 // why\n\t_ = x\n}\n"})
	got, failure := loader.Checker{Dir: dir}.Check(t.Context(), []string{"./..."})
	want := port.Finding{
		Path: filepath.Join("p", "p.go"), Line: 4, Col: 9, Rule: "comments",
		Reason: "no tool reads this comment: let the code explain itself and a test carry the why",
	}
	if failure != nil || len(got) != 1 || got[0] != want {
		t.Errorf("Check = %v, %v, want %v", got, failure, want)
	}
}

func TestTheLoaderChecksTestFilesOnceEach(t *testing.T) {
	dir := module(t, map[string]string{
		"p/p.go":             "package p\n\nfunc F() {}\n",
		"p/p_test.go":        "package p\n\nimport \"testing\"\n\n// in the package's tests\nfunc TestF(t *testing.T) {}\n",
		"p/external_test.go": "package p_test\n\nimport \"testing\"\n\n// in its external tests\nfunc TestG(t *testing.T) {}\n",
	})
	got, failure := loader.Checker{Dir: dir}.Check(t.Context(), []string{"./p"})
	if failure != nil || len(got) != 2 {
		t.Fatalf("Check = %v, %v, want two findings, one for each test file", got, failure)
	}
	if got[0].Path != filepath.Join("p", "external_test.go") || got[1].Path != filepath.Join("p", "p_test.go") {
		t.Errorf("Check = %v, want external_test.go then p_test.go", got)
	}
}

func TestAFileOutsideTheFolderKeepsItsPath(t *testing.T) {
	outer := module(t, map[string]string{"p/p.go": "package p\n\n// here\nfunc F() {}\n"})
	inner := filepath.Join(outer, "p")
	got, failure := loader.Checker{Dir: inner}.Check(t.Context(), []string{outer + "/p"})
	if failure != nil || len(got) != 1 || got[0].Path != "p.go" {
		t.Errorf("Check = %v, %v, want one finding of p.go", got, failure)
	}
}

func TestAPatternOutsideTheFolderIsGivenWhole(t *testing.T) {
	outer := module(t, map[string]string{"p/p.go": "package p\n\n// here\nfunc F() {}\n"})
	elsewhere := filepath.Join(outer, "sibling")
	if err := os.Mkdir(elsewhere, 0o750); err != nil {
		t.Fatal(err)
	}
	got, failure := loader.Checker{Dir: elsewhere}.Check(t.Context(), []string{"example.test/m/p"})
	want := filepath.Join(outer, "p", "p.go")
	if failure != nil || len(got) != 1 || got[0].Path != want {
		t.Errorf("Check = %v, %v, want one finding at %s", got, failure, want)
	}
}

func TestAPatternThatMatchesNoPackageIsFailed(t *testing.T) {
	dir := module(t, map[string]string{"p/p.go": "package p\n"})
	got, failure := loader.Checker{Dir: dir}.Check(t.Context(), []string{"./nothing/..."})
	if _, ok := failure.(*port.Failed); !ok || got != nil {
		t.Errorf("Check = %v, %v, want *port.Failed and no findings", got, failure)
	}
}

func TestAFolderThatDoesNotExistIsFailed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	got, failure := loader.Checker{Dir: missing}.Check(t.Context(), []string{"./..."})
	if _, ok := failure.(*port.Failed); !ok || got != nil {
		t.Errorf("Check = %v, %v, want *port.Failed and no findings", got, failure)
	}
}

func TestAModuleWithNoPackageIsFailed(t *testing.T) {
	dir := module(t, map[string]string{})
	got, failure := loader.Checker{Dir: dir}.Check(t.Context(), []string{"./..."})
	failed, ok := failure.(*port.Failed)
	if !ok || got != nil || failed.Reason != "the patterns match no package" {
		t.Errorf("Check = %v, %v, want *port.Failed saying the patterns match no package", got, failure)
	}
}

func TestACgoPackageKeepsItsLicenseHeaderAndPreamble(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	dir := module(t, map[string]string{
		"c/c.go": `//go:build cgo

// Copyright 2026 A.
// All rights reserved.

package c

// #include <stdlib.h>
import "C"

func F() { _ = C.int(1) }

// here
func G() {}
`,
	})
	got, failure := loader.Checker{Dir: dir}.Check(t.Context(), []string{"./c"})
	if failure != nil || len(got) != 1 || got[0].Path != filepath.Join("c", "c.go") || got[0].Line != 13 || got[0].Col != 1 {
		t.Errorf("Check = %v, %v, want one finding at c/c.go:13:1", got, failure)
	}
}
