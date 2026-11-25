package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"strings"
	"unicode/utf8"

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
	visitTestFiles(pass, func(file *ast.File) {
		ast.Inspect(file, func(node ast.Node) bool {
			visit(node)
			return true
		})
	})
}

func visitTestFiles(pass *analysis.Pass, visit func(*ast.File)) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.PositionFor(file.Pos(), false).Filename, "_test.go") {
			visit(file)
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

func immediateLiteral(call *ast.CallExpr, parent ast.Node) *ast.FuncLit {
	switch parent.(type) {
	case *ast.DeferStmt, *ast.GoStmt:
		return nil
	}
	literal, _ := ast.Unparen(call.Fun).(*ast.FuncLit)
	return literal
}

// pointerAliases records possible sources for named pointers across assignments.
// A call through an alias can write any of its targets, including after a branch.
type pointerAliases map[types.Object][]ast.Expr

func indexPointerAliases(info *types.Info, files []*ast.File) pointerAliases {
	aliases := make(pointerAliases)
	record := func(target ast.Expr, source ast.Expr) {
		name, ok := ast.Unparen(target).(*ast.Ident)
		if !ok {
			return
		}
		object, ok := info.ObjectOf(name).(*types.Var)
		if !ok {
			return
		}
		if _, pointer := object.Type().Underlying().(*types.Pointer); pointer {
			aliases[object] = append(aliases[object], source)
		}
	}
	for _, file := range files {
		for node := range ast.Preorder(file) {
			switch stmt := node.(type) {
			case *ast.AssignStmt:
				if len(stmt.Lhs) == len(stmt.Rhs) {
					for i, target := range stmt.Lhs {
						record(target, stmt.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				if len(stmt.Names) == len(stmt.Values) {
					for i, name := range stmt.Names {
						record(name, stmt.Values[i])
					}
				}
			}
		}
	}
	return aliases
}

func (aliases pointerAliases) visitTargets(info *types.Info, args []ast.Expr, visit func(types.Object)) {
	seen := make(map[types.Object]bool)
	var resolve func(ast.Expr)
	resolve = func(expr ast.Expr) {
		if target := addressedVariable(info, expr); target != nil {
			visit(target)
			return
		}
		if name, ok := ast.Unparen(expr).(*ast.Ident); ok {
			object := info.Uses[name]
			if seen[object] {
				return
			}
			seen[object] = true
			for _, source := range aliases[object] {
				resolve(source)
			}
		}
	}
	for _, arg := range args {
		resolve(arg)
	}
}

// After evaluating the arguments, a call can write through their addresses.
func forgetCallWrites[V any](info *types.Info, aliases pointerAliases, call *ast.CallExpr, parent ast.Node, tracked map[types.Object]V) {
	callee := info.Types[call.Fun]
	if _, deferred := parent.(*ast.DeferStmt); deferred || callee.IsType() || callee.IsBuiltin() {
		return
	}
	aliases.visitTargets(info, call.Args, func(object types.Object) { delete(tracked, object) })
}

func addressedVariable(info *types.Info, expr ast.Expr) types.Object {
	if address, ok := ast.Unparen(expr).(*ast.UnaryExpr); ok && address.Op == token.AND {
		if name, ok := ast.Unparen(address.X).(*ast.Ident); ok {
			return info.Uses[name]
		}
	}
	return nil
}

// deferredWrites identifies variables that a function's defers may change on exit.
func deferredWrites(info *types.Info, aliases pointerAliases, body *ast.BlockStmt) map[types.Object]bool {
	writes := make(map[types.Object]bool)
	ast.PreorderStack(body, nil, func(node ast.Node, _ []ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.FuncLit:
			return false // Its defers belong to a separate call.
		case *ast.DeferStmt:
			if literal, ok := ast.Unparen(stmt.Call.Fun).(*ast.FuncLit); ok {
				maps.Copy(writes, mutableVariables(info, literal.Body))
			}
			aliases.visitTargets(info, stmt.Call.Args, func(object types.Object) { writes[object] = true })
		}
		return true
	})
	return writes
}

func invokesLiteral(literal *ast.FuncLit, parents []ast.Node) bool {
	// Skip parentheses to check whether the literal is the callee.
	for i := len(parents) - 1; i >= 0; i-- {
		if _, paren := parents[i].(*ast.ParenExpr); paren {
			continue
		}
		call, ok := parents[i].(*ast.CallExpr)
		if !ok {
			break
		}
		var enclosing ast.Node
		if i > 0 {
			enclosing = parents[i-1]
		}
		return immediateLiteral(call, enclosing) == literal
	}
	return false
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

const unknownIterations = -1

type loopBounds struct {
	maxIterations int64
	maySkip       bool
}

func forBounds(info *types.Info, cond ast.Expr) loopBounds {
	value := info.Types[cond].Value
	if value != nil && !constant.BoolVal(value) {
		return loopBounds{maySkip: true}
	}
	return loopBounds{maxIterations: unknownIterations, maySkip: cond != nil && value == nil}
}

func rangeBounds(info *types.Info, expr ast.Expr) loopBounds {
	if value := info.Types[expr].Value; value != nil {
		switch value.Kind() {
		case constant.Int:
			if constant.Sign(value) <= 0 {
				return loopBounds{maySkip: true}
			}
			if count, exact := constant.Int64Val(value); exact {
				return loopBounds{maxIterations: count}
			}
			return loopBounds{maxIterations: unknownIterations}
		case constant.String:
			count := utf8.RuneCountInString(constant.StringVal(value))
			return loopBounds{maxIterations: int64(count), maySkip: count == 0}
		}
	}
	underlying := info.TypeOf(expr).Underlying()
	if pointer, ok := underlying.(*types.Pointer); ok {
		underlying = pointer.Elem().Underlying()
	}
	if array, ok := underlying.(*types.Array); ok {
		return loopBounds{maxIterations: array.Len(), maySkip: array.Len() == 0}
	}
	return loopBounds{maxIterations: unknownIterations, maySkip: true}
}

func unlabel(stmt ast.Stmt) ast.Stmt {
	for label, ok := stmt.(*ast.LabeledStmt); ok; label, ok = stmt.(*ast.LabeledStmt) {
		stmt = label.Stmt
	}
	return stmt
}

// A select evaluates all communication operands in source order before choosing a clause.
func visitSelectOperands(body *ast.BlockStmt, visit func(ast.Expr)) {
	for _, stmt := range body.List {
		switch comm := stmt.(*ast.CommClause).Comm.(type) {
		case *ast.SendStmt:
			visit(comm.Chan)
			visit(comm.Value)
		case *ast.ExprStmt:
			visit(comm.X)
		case *ast.AssignStmt:
			visit(comm.Rhs[0])
		}
	}
}

// Only a selected receive assignment evaluates its targets and writes the received values.
func visitReceiveTargets(comm ast.Stmt, visit func(ast.Expr), clear func(*ast.Ident)) {
	assignment, ok := comm.(*ast.AssignStmt)
	if !ok {
		return
	}
	for _, lhs := range assignment.Lhs {
		visit(lhs)
	}
	for _, lhs := range assignment.Lhs {
		if name, ok := ast.Unparen(lhs).(*ast.Ident); ok {
			clear(name)
		}
	}
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
