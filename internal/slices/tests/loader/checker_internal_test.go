package loader

import (
	"context"
	"errors"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

func TestCheckPackagesDiscardsFindingsOnAnalyzerFailure(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("p_test.go", -1, 1)
	pkgs := []*packages.Package{{Fset: fset}}
	cause := errors.New("analysis failed")
	rule := &analysis.Analyzer{Run: func(pass *analysis.Pass) (any, error) {
		pass.Report(analysis.Diagnostic{Pos: file.Pos(0), Message: "partial finding"})
		return nil, cause
	}}

	found, failure := checkPackages(t.Context(), pkgs, t.TempDir(), []*analysis.Analyzer{rule})
	var failed *port.Failed
	if len(found) != 0 || !errors.As(failure, &failed) || failed.Reason != cause.Error() {
		t.Errorf("checkPackages() = %v, %v, want no findings and failure %q", found, failure, cause)
	}
}

func TestCheckPackagesStopsBeforeNextPackageOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fset := token.NewFileSet()
	file := fset.AddFile("p_test.go", -1, 1)
	pkgs := []*packages.Package{
		{Fset: fset},
		{Errors: []packages.Error{{Msg: "a cancelled check must not reach this package"}}},
	}
	rule := &analysis.Analyzer{Run: func(pass *analysis.Pass) (any, error) {
		pass.Report(analysis.Diagnostic{Pos: file.Pos(0), Message: "partial finding"})
		cancel()
		return nil, nil
	}}

	found, failure := checkPackages(ctx, pkgs, t.TempDir(), []*analysis.Analyzer{rule})
	var cancelled *port.Cancelled
	if len(found) != 0 || !errors.As(failure, &cancelled) {
		t.Errorf("checkPackages() = %v, %v, want no findings and a cancellation", found, failure)
	}
}
