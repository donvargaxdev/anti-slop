package tests

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Sleep = &analysis.Analyzer{
	Name: "tests_sleep",
	Doc:  "report time.Sleep in Go test code",
	Run: func(pass *analysis.Pass) (any, error) {
		visitTests(pass, func(node ast.Node) {
			if call, ok := node.(*ast.CallExpr); ok && packageCall(pass.TypesInfo, call, "time", "Sleep") {
				report(pass, call, "time.Sleep in test code")
			}
		})
		return nil, nil
	},
}

var Environment = &analysis.Analyzer{
	Name: "tests_environment",
	Doc:  "report direct process environment changes in Go test code",
	Run: func(pass *analysis.Pass) (any, error) {
		visitTests(pass, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			if packageCall(pass.TypesInfo, call, "os", "Setenv", "Unsetenv") {
				report(pass, call, "os."+calledFunc(pass.TypesInfo, call).Name()+" changes the process environment in test code")
			}
		})
		return nil, nil
	},
}

func visitTests(pass *analysis.Pass, visit func(ast.Node)) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.PositionFor(file.Pos(), false).Filename, "_test.go") {
			ast.Inspect(file, func(node ast.Node) bool {
				visit(node)
				return true
			})
		}
	}
}

func calledFunc(info *types.Info, call *ast.CallExpr) *types.Func {
	fun := ast.Unparen(call.Fun)
	switch indexed := fun.(type) {
	case *ast.IndexExpr:
		fun = ast.Unparen(indexed.X)
	case *ast.IndexListExpr:
		fun = ast.Unparen(indexed.X)
	}
	var ident *ast.Ident
	switch fun := fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	}
	fn, _ := info.Uses[ident].(*types.Func)
	return fn
}

func packageCall(info *types.Info, call *ast.CallExpr, path string, names ...string) bool {
	fn := calledFunc(info, call)
	return fn != nil && fn.Signature().Recv() == nil && namedInPackage(fn, path, names)
}

func testingMethod(info *types.Info, call *ast.CallExpr, names ...string) bool {
	fn := calledFunc(info, call)
	return fn != nil && fn.Signature().Recv() != nil && namedInPackage(fn, "testing", names)
}

func namedInPackage(fn *types.Func, path string, names []string) bool {
	if fn.Pkg() == nil || fn.Pkg().Path() != path {
		return false
	}
	for _, name := range names {
		if fn.Name() == name {
			return true
		}
	}
	return false
}

func report(pass *analysis.Pass, node ast.Node, reason string) {
	pass.Report(analysis.Diagnostic{Pos: node.Pos(), Category: "tests", Message: reason})
}

// contains checks an expression or statement without entering another function body.
func contains(root ast.Node, match func(ast.Node) bool) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil || found {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		found = match(node)
		return !found
	})
	return found
}
