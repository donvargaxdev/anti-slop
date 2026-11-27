// Package recursion reports direct and short cycles among named Go functions
// and methods in the same package.
package recursion

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	name           = "recursion"
	maxCycleLength = 3
)

var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "report named functions and methods that call themselves within three calls in the same package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var functions []*types.Func
	graph := make(map[*types.Func][]*types.Func)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			caller := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			functions = append(functions, caller)
			graph[caller] = localCalls(pass, fn.Body)
		}
	}
	for _, fn := range functions {
		length := cycleLength(graph, fn)
		if length == 0 {
			continue
		}
		reason := fn.Name() + " calls itself"
		if length > 1 {
			reason += " through a cycle"
		}
		pass.Report(analysis.Diagnostic{Pos: fn.Pos(), Category: name, Message: reason})
	}
	return nil, nil
}

func localCalls(pass *analysis.Pass, body *ast.BlockStmt) []*types.Func {
	var calls []*types.Func
	ast.Inspect(body, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.FuncLit:
			// A literal has its own body and may never be called.
			return false
		case *ast.CallExpr:
			callee := calledFunction(pass.TypesInfo, expr.Fun)
			if callee != nil && callee.Pkg() == pass.Pkg {
				calls = append(calls, callee.Origin())
			}
		}
		return true
	})
	return calls
}

func calledFunction(info *types.Info, expr ast.Expr) *types.Func {
	// Unwrap explicit type arguments. An ordinary index resolves to a
	// variable or field, which the function assertion below excludes.
	switch indexed := ast.Unparen(expr).(type) {
	case *ast.IndexExpr:
		expr = indexed.X
	case *ast.IndexListExpr:
		expr = indexed.X
	}

	var name *ast.Ident
	switch callee := ast.Unparen(expr).(type) {
	case *ast.Ident:
		name = callee
	case *ast.SelectorExpr:
		name = callee.Sel
	}
	fn, _ := info.Uses[name].(*types.Func)
	return fn
}

// cycleLength searches in order of distance so each expanded function is
// reached by its shortest path from start.
func cycleLength(graph map[*types.Func][]*types.Func, start *types.Func) int {
	frontier := []*types.Func{start}
	seen := make(map[*types.Func]bool)
	for depth := 1; depth <= maxCycleLength && len(frontier) > 0; depth++ {
		var next []*types.Func
		for _, caller := range frontier {
			for _, callee := range graph[caller] {
				if callee == start {
					return depth
				}
				if !seen[callee] {
					seen[callee] = true
					next = append(next, callee)
				}
			}
		}
		frontier = next
	}
	return 0
}
