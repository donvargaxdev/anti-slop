// Package tests reports sleeps and raw environment changes in Go test files.
package tests

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "tests",
	Doc:  "report calls to time.Sleep, os.Setenv and os.Unsetenv in Go test files",
	Run: func(pass *analysis.Pass) (any, error) {
		Report(pass)
		return nil, nil
	},
}

var reasons = map[string]string{
	"time.Sleep":  "avoid time.Sleep in tests",
	"os.Setenv":   "use t.Setenv instead of os.Setenv",
	"os.Unsetenv": "avoid os.Unsetenv in tests",
}

// Report emits a diagnostic for each call whose callee resolves to a flagged
// standard-library function. The physical filename identifies test files.
func Report(pass *analysis.Pass) {
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		for node := range ast.Preorder(file) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				continue
			}
			if reason := reasonFor(pass.TypesInfo, call.Fun); reason != "" {
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(), Category: "tests", Message: reason,
				})
			}
		}
	}
}

func reasonFor(info *types.Info, callee ast.Expr) string {
	var name *ast.Ident
	switch expr := ast.Unparen(callee).(type) {
	case *ast.Ident:
		name = expr
	case *ast.SelectorExpr:
		name = expr.Sel
	}
	fn, ok := info.Uses[name].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return ""
	}
	return reasons[fn.Pkg().Path()+"."+fn.Name()]
}
