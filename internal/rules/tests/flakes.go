package tests

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Flakes = &analysis.Analyzer{
	Name: "tests_flakes",
	Doc:  "report retries and failure-dependent skips in Go test code",
	Run:  runFlakes,
}

type flakeChecks struct {
	info     *types.Info
	outcomes map[types.Object]bool
}

func runFlakes(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	checks := flakeChecks{info: info, outcomes: make(map[types.Object]bool)}
	skips := make(map[token.Pos]bool)
	visitTests(pass, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.ForStmt:
			if checks.retriesInHeader(stmt) || checks.retriesLoop(stmt.Body) {
				report(pass, stmt, "retrying a check or test body")
			}
		case *ast.RangeStmt:
			if kind, ok := info.TypeOf(stmt.X).Underlying().(*types.Basic); ok &&
				kind.Info()&types.IsInteger != 0 && checks.retriesLoop(stmt.Body) {
				report(pass, stmt, "retrying a check or test body")
			}
		case *ast.AssignStmt:
			checks.rememberOutcome(stmt)
		case *ast.IfStmt:
			if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
				checks.rememberOutcome(assign)
			}
			if branch := checks.failureBranch(stmt, true, nil); branch != nil {
				contains(branch, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok && !skips[call.Pos()] &&
						testingMethod(info, call, "Skip", "Skipf", "SkipNow") {
						report(pass, call, "skipping after a failure")
						skips[call.Pos()] = true
					}
					return false
				})
			}
		case *ast.CallExpr:
			if wrapsTestInRetry(info, stmt) {
				report(pass, stmt, "retrying a test body")
			}
		}
	})
	return nil, nil
}

// rememberOutcome follows simple assignments of testing status to a local
// variable, so storing t.Run or t.Failed does not hide a failure-dependent skip.
func (c flakeChecks) rememberOutcome(stmt *ast.AssignStmt) {
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		object := c.info.ObjectOf(ident)
		if len(stmt.Lhs) == len(stmt.Rhs) && booleanResult(c.info, stmt.Rhs[i]) {
			if c.failureCondition(stmt.Rhs[i], true, nil) {
				c.outcomes[object] = true
				continue
			}
			if c.failureCondition(stmt.Rhs[i], false, nil) {
				c.outcomes[object] = false
				continue
			}
		}
		delete(c.outcomes, object)
	}
}

func (c flakeChecks) retriesLoop(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		switch node := node.(type) {
		case nil, *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		case *ast.IfStmt:
			found = exitsAttempt(c.failureBranch(node, false, body), false) ||
				exitsAttempt(c.failureBranch(node, true, body), true)
		}
		return !found
	})
	return found
}

func (c flakeChecks) retriesInHeader(loop *ast.ForStmt) bool {
	if loop.Cond == nil {
		return false
	}
	if post, ok := loop.Post.(*ast.AssignStmt); ok {
		return c.failureCondition(loop.Cond, true, post)
	}
	return false
}

func (c flakeChecks) failureBranch(stmt *ast.IfStmt, failure bool, attempt ast.Node) ast.Stmt {
	if c.failureCondition(stmt.Cond, failure, attempt) {
		return stmt.Body
	}
	if c.failureCondition(stmt.Cond, !failure, attempt) {
		return stmt.Else
	}
	return nil
}

// failureCondition recognizes a check whose success or failure determines this
// branch. In a retry loop a result must come from inside the loop.
func (c flakeChecks) failureCondition(condition ast.Expr, failure bool, attempt ast.Node) bool {
	switch expr := ast.Unparen(condition).(type) {
	case *ast.UnaryExpr:
		return expr.Op == token.NOT && c.failureCondition(expr.X, !failure, attempt)
	case *ast.BinaryExpr:
		return c.failureComparison(expr, failure, attempt)
	case *ast.CallExpr:
		switch {
		case testingMethod(c.info, expr, "Failed"), packageCall(c.info, expr, "errors", "Is", "As"):
			return failure
		case testingMethod(c.info, expr, "Run"):
			return !failure
		case attempt != nil:
			return !failure && booleanResult(c.info, expr) && !packageCall(c.info, expr, "testing", "Short")
		}
	case *ast.Ident:
		if failed, known := c.outcomes[c.info.ObjectOf(expr)]; known && attemptResult(c.info, expr, attempt) {
			return failure == failed
		}
		return !failure && attempt != nil && attemptResult(c.info, expr, attempt) && booleanResult(c.info, expr)
	}
	return false
}

func (c flakeChecks) failureComparison(expr *ast.BinaryExpr, failure bool, attempt ast.Node) bool {
	switch expr.Op {
	case token.LAND, token.LOR:
		either := (expr.Op == token.LAND) == failure
		left := c.failureCondition(expr.X, failure, attempt)
		right := c.failureCondition(expr.Y, failure, attempt)
		if either {
			return left || right
		}
		return left && right
	case token.EQL, token.NEQ:
		if (expr.Op == token.NEQ) != failure {
			return false
		}
		return errorComparedWithNil(c.info, expr.X, expr.Y, attempt) ||
			errorComparedWithNil(c.info, expr.Y, expr.X, attempt)
	}
	return false
}

func errorComparedWithNil(info *types.Info, value, other ast.Expr, attempt ast.Node) bool {
	nilIdent, ok := ast.Unparen(other).(*ast.Ident)
	if !ok || info.Uses[nilIdent] != types.Universe.Lookup("nil") {
		return false
	}
	typ := info.TypeOf(value)
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return typ != nil && types.Implements(typ, errorType) && attemptResult(info, value, attempt)
}

func attemptResult(info *types.Info, value ast.Expr, attempt ast.Node) bool {
	if attempt == nil {
		return true
	}
	switch expr := ast.Unparen(value).(type) {
	case *ast.CallExpr:
		return true
	case *ast.Ident:
		object := info.ObjectOf(expr)
		return object != nil && (attempt.Pos() < object.Pos() && object.Pos() < attempt.End() ||
			contains(attempt, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if !ok {
					return false
				}
				for _, lhs := range assign.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && info.ObjectOf(ident) == object {
						return true
					}
				}
				return false
			}))
	}
	return false
}

func booleanResult(info *types.Info, expr ast.Expr) bool {
	typ := info.TypeOf(expr)
	return typ != nil && types.Identical(typ.Underlying(), types.Typ[types.Bool])
}

func exitsAttempt(branch ast.Stmt, failure bool) bool {
	if branch == nil {
		return false
	}
	block, ok := branch.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return false
	}
	switch last := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return !failure
	case *ast.BranchStmt:
		return last.Tok == token.BREAK && !failure || last.Tok == token.CONTINUE && failure
	}
	return false
}

func wrapsTestInRetry(info *types.Info, call *ast.CallExpr) bool {
	fn := calledFunc(info, call)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	name := strings.ToLower(fn.Name())
	path := fn.Pkg().Path()
	if name != "retry" && name != "retrytest" && name != "retrysubtest" &&
		path != "retry" && !strings.HasSuffix(path, "/retry") {
		return false
	}
	for _, arg := range call.Args {
		if typ := info.TypeOf(arg); typ != nil {
			if sig, ok := typ.Underlying().(*types.Signature); ok && sig.Params().Len() > 0 &&
				isTestingT(sig.Params().At(0).Type()) {
				return true
			}
		}
		body, ok := ast.Unparen(arg).(*ast.FuncLit)
		if ok && contains(body.Body, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			return ok && isTestingT(info.TypeOf(ident))
		}) {
			return true
		}
	}
	return false
}

func isTestingT(typ types.Type) bool {
	pointer, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "T"
}
