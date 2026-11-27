// Package loader loads Go packages and checks their declarations.
package loader

import (
	"context"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/rules/unitslast"
)

type Checker struct{ Dir string }

func (c Checker) Check(ctx context.Context, patterns []string) (found []port.Finding, failure port.CheckFailure) {
	defer func() {
		if ctx.Err() != nil {
			found, failure = nil, &port.Cancelled{}
		}
	}()

	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     c.Dir,
		Fset:    fset,
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
		if len(pkg.Errors) > 0 {
			return nil, &port.Failed{Reason: pkg.Errors[0].Error()}
		}
		// The analyzer only reports diagnostics and always returns nil.
		_, _ = unitslast.Analyzer.Run(&analysis.Pass{
			Analyzer:  unitslast.Analyzer,
			Fset:      fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report: func(d analysis.Diagnostic) {
				at := fset.PositionFor(d.Pos, false)
				found = append(found, port.Finding{
					Path: relative(root, at.Filename), Line: at.Line, Col: at.Column, Rule: d.Category, Reason: d.Message,
				})
			},
		})
	}
	return port.Merge(nil, found), nil
}

func relative(root, name string) string {
	rel, _ := filepath.Rel(root, name)
	if !filepath.IsLocal(rel) {
		return name
	}
	return rel
}
