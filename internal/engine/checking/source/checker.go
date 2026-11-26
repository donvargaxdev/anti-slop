// Package source loads and checks the original Go source files of packages.
package source

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

// Checker is the port's implementation over the go command. Dir is the folder
// patterns are read in and paths are given relative to; "" is the working
// folder.
type Checker struct {
	Dir    string
	Report func(*token.FileSet, []*ast.File, func(analysis.Diagnostic))
}

// Check loads the packages of patterns with their tests, parses the source
// files they list, and gives each finding once. The original source preserves
// cgo license headers and preambles. A package that cannot be loaded or parsed
// is a Failed failure with the first error, and a done ctx is Cancelled.
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
		Mode:    packages.NeedName | packages.NeedFiles,
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
		failure := c.reportSources(ctx, fset, pkg.GoFiles, func(d analysis.Diagnostic) {
			at := fset.PositionFor(d.Pos, false)
			found = append(found, port.Finding{
				Path: relative(root, at.Filename), Line: at.Line, Col: at.Column, Rule: d.Category, Reason: d.Message,
			})
		})
		if failure != nil {
			return nil, failure
		}
	}
	return port.Merge(nil, found), nil
}

func (c Checker) reportSources(ctx context.Context, fset *token.FileSet, names []string, report func(analysis.Diagnostic)) port.CheckFailure {
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return &port.Failed{Reason: err.Error()}
		}
		c.Report(fset, []*ast.File{file}, report)
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
