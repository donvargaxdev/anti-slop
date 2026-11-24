// Package loader is the comments slice's adapter of its port: it loads the
// packages a pattern names with go list and runs the rule over their syntax.
package loader

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/donvargaxdev/anti-slop/internal/rules/comments"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
)

// Checker is the port's implementation over the go command. Dir is the folder
// patterns are read in and paths are given relative to; "" is the working
// folder.
type Checker struct{ Dir string }

// Check loads the packages of patterns with their tests, parses the source
// files they list, and gives each finding once. The original source preserves
// cgo license headers and preambles. A package that cannot be loaded or parsed
// is a Failed failure with the first error, and a done ctx is Cancelled.
func (c Checker) Check(ctx context.Context, patterns []string) ([]port.Finding, port.CheckFailure) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     c.Dir,
		Fset:    fset,
		Tests:   true,
		Mode:    packages.NeedName | packages.NeedFiles,
	}, patterns...)
	if ctx.Err() != nil {
		return nil, &port.Cancelled{}
	}
	if err != nil {
		return nil, &port.Failed{Reason: err.Error()}
	}
	if len(pkgs) == 0 {
		return nil, &port.Failed{Reason: "the patterns match no package"}
	}
	root, _ := filepath.Abs(c.Dir)
	var found []port.Finding
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, &port.Failed{Reason: pkg.Errors[0].Error()}
		}
		failure := reportSources(ctx, fset, pkg.GoFiles, func(d analysis.Diagnostic) {
			at := fset.Position(d.Pos)
			found = append(found, port.Finding{
				Path: relative(root, at.Filename), Line: at.Line, Col: at.Column, Rule: d.Category, Reason: d.Message,
			})
		})
		if failure != nil {
			return nil, failure
		}
	}
	found = port.Merge(nil, found)
	if ctx.Err() != nil {
		return nil, &port.Cancelled{}
	}
	return found, nil
}

func reportSources(ctx context.Context, fset *token.FileSet, names []string, report func(analysis.Diagnostic)) port.CheckFailure {
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			return &port.Failed{Reason: err.Error()}
		}
		comments.Report([]*ast.File{file}, report)
	}
	if ctx.Err() != nil {
		return &port.Cancelled{}
	}
	return nil
}

// relative is name below root as a relative path, or name as it is when it
// lies elsewhere.
func relative(root, name string) string {
	rel, err := filepath.Rel(root, name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return name
	}
	return rel
}
