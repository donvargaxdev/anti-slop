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
// files they list as written, never the rewritten input of cgo, so a license
// header stays the first comment of its file, and gives each finding of the
// rule once. A package that cannot be loaded or parsed is a Failed failure with
// the first error, and a done ctx is Cancelled.
// parsed is a Failed failure with the loader's first error, and a done ctx is
// Cancelled.
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
		files := make([]*ast.File, 0, len(pkg.GoFiles))
		for _, name := range pkg.GoFiles {
			file, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				return nil, &port.Failed{Reason: err.Error()}
			}
			files = append(files, file)
		}
		var reported []port.Finding
		comments.Report(files, func(d analysis.Diagnostic) {
			at := fset.Position(d.Pos)
			reported = append(reported, port.Finding{
				Path: relative(root, at.Filename), Line: at.Line, Col: at.Column, Rule: d.Category, Reason: d.Message,
			})
		})
		found = port.Merge(found, reported)
	}
	return found, nil
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
