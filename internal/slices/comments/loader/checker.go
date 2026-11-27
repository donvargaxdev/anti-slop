// Package loader loads Go packages for rules that only need their syntax.
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

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/rules/comments"
)

// Checker is the port's implementation over the go command. Dir is the folder
// patterns are read in and paths are given relative to; "" is the working
// folder. A nil Report selects the comments rule.
type Checker struct {
	Dir    string
	Report SourceReporter
}

// SourceReporter checks the source files listed for one package.
type SourceReporter func(context.Context, *token.FileSet, []string, func(analysis.Diagnostic)) port.CheckFailure

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
	checkSources := c.Report
	if checkSources == nil {
		checkSources = reportSources
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, &port.Failed{Reason: pkg.Errors[0].Error()}
		}
		failure := checkSources(ctx, fset, pkg.GoFiles, func(d analysis.Diagnostic) {
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

// ReportSources adapts a rule that works on source syntax into a SourceReporter.
func ReportSources(analyzer *analysis.Analyzer) SourceReporter {
	return func(ctx context.Context, fset *token.FileSet, names []string, report func(analysis.Diagnostic)) port.CheckFailure {
		for _, name := range names {
			if ctx.Err() != nil {
				break
			}
			file, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				return &port.Failed{Reason: err.Error()}
			}
			pass := &analysis.Pass{Analyzer: analyzer, Fset: fset, Files: []*ast.File{file}, Report: report}
			if _, err := analyzer.Run(pass); err != nil {
				return &port.Failed{Reason: err.Error()}
			}
		}
		if ctx.Err() != nil {
			return &port.Cancelled{}
		}
		return nil
	}
}

func reportSources(ctx context.Context, fset *token.FileSet, names []string, report func(analysis.Diagnostic)) port.CheckFailure {
	return ReportSources(comments.Analyzer)(ctx, fset, names, report)
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
