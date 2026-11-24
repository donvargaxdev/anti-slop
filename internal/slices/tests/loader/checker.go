package loader

import (
	"context"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/rules/tests"
)

type Checker struct{ Dir string }

func (c Checker) Check(ctx context.Context, patterns []string) (findings []port.Finding, failure port.CheckFailure) {
	defer func() {
		if ctx.Err() != nil {
			findings, failure = nil, &port.Cancelled{}
		}
	}()

	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     c.Dir,
		Tests:   true,
		Mode:    packages.LoadSyntax,
	}, patterns...)
	if err != nil {
		return nil, &port.Failed{Reason: err.Error()}
	}
	if len(pkgs) == 0 {
		return nil, &port.Failed{Reason: "the patterns match no package"}
	}

	root, _ := filepath.Abs(c.Dir)
	for _, pkg := range pkgs {
		if ctx.Err() != nil {
			break
		}
		if len(pkg.Errors) > 0 {
			return nil, &port.Failed{Reason: pkg.Errors[0].Error()}
		}
		if err := checkPackage(pkg, root, &findings); err != nil {
			return nil, &port.Failed{Reason: err.Error()}
		}
	}
	return port.Merge(nil, findings), nil
}

func checkPackage(pkg *packages.Package, root string, findings *[]port.Finding) error {
	for _, analyzer := range []*analysis.Analyzer{tests.Sleep, tests.Polling, tests.Environment, tests.Flakes} {
		pass := &analysis.Pass{
			Analyzer:  analyzer,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report: func(d analysis.Diagnostic) {
				at := pkg.Fset.PositionFor(d.Pos, false)
				path := at.Filename
				if rel, err := filepath.Rel(root, path); err == nil && filepath.IsLocal(rel) {
					path = rel
				}
				*findings = append(*findings, port.Finding{
					Path: path, Line: at.Line, Col: at.Column, Rule: d.Category, Reason: d.Message,
				})
			},
		}
		if _, err := analyzer.Run(pass); err != nil {
			return err
		}
	}
	return nil
}
