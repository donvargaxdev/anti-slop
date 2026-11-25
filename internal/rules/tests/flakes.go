package tests

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"slices"
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
	outcomes map[types.Object]outcomePolarity
}

type outcomePolarity uint8

const (
	failsWhenTrue outcomePolarity = 1 << iota
	failsWhenFalse
)

func runFlakes(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	checks := flakeChecks{info: info, outcomes: make(map[types.Object]outcomePolarity)}
	skips := make(map[token.Pos]bool)
	visitTestFiles(pass, func(file *ast.File) {
		checks.walk(file, func(checks flakeChecks, node, _ ast.Node) bool {
			checks.reportFinding(pass, node, skips)
			return true
		})
	})
	return nil, nil
}

func (c flakeChecks) reportFinding(pass *analysis.Pass, node ast.Node, skips map[token.Pos]bool) {
	switch stmt := node.(type) {
	case *ast.ForStmt:
		if c.retriesInHeader(stmt) || c.retriesLoop(stmt.Body) {
			report(pass, stmt, "retrying a check or test body")
		}
	case *ast.RangeStmt:
		if c.retriesLoop(stmt.Body) {
			report(pass, stmt, "retrying a check or test body")
		}
	case *ast.IfStmt:
		for _, branch := range c.outcomeBranches(stmt, true, nil) {
			if branch == nil {
				continue
			}
			contains(branch, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok && !skips[call.Pos()] &&
					testingMethod(c.info, call, "Skip", "Skipf", "SkipNow") {
					report(pass, call, "skipping after a failure")
					skips[call.Pos()] = true
				}
				return false
			})
		}
	case *ast.CallExpr:
		if wrapsTestInRetry(c.info, stmt) {
			report(pass, stmt, "retrying a test body")
		}
	}
}

// walk tracks stored outcomes in source order and joins alternative branches.
func (c flakeChecks) walk(root ast.Node, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	if root == nil {
		return
	}
	ast.PreorderStack(root, nil, func(node ast.Node, parents []ast.Node) bool {
		var parent ast.Node
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		}
		if stmt, ok := node.(*ast.IfStmt); ok {
			c.walk(stmt.Init, visit)
			if visit(c, stmt, parent) {
				c.walk(stmt.Cond, visit)
				then := c
				then.outcomes = maps.Clone(c.outcomes)
				then.walk(stmt.Body, visit)
				c.walk(stmt.Else, visit)
				mergeOutcomes(c.outcomes, then.outcomes)
			}
			return false
		}
		if !visit(c, node, parent) {
			return false
		}
		switch stmt := node.(type) {
		case *ast.SwitchStmt:
			c.walk(stmt.Init, visit)
			c.walk(stmt.Tag, visit)
			c.walkCases(stmt.Body, visit)
			return false
		case *ast.TypeSwitchStmt:
			c.walk(stmt.Init, visit)
			c.walk(stmt.Assign, visit)
			c.walkCases(stmt.Body, visit)
			return false
		case *ast.SelectStmt:
			c.walkCommunications(stmt.Body, visit)
			return false
		case *ast.FuncLit:
			body := c
			body.outcomes = maps.Clone(c.outcomes)
			body.walk(stmt.Body, visit)
			return false
		case *ast.AssignStmt:
			outcomes := c.assignmentOutcomes(stmt.Rhs, len(stmt.Lhs))
			for i, lhs := range stmt.Lhs {
				if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					c.storeOutcome(ident, outcomes[i])
				}
			}
		case *ast.ValueSpec:
			outcomes := c.assignmentOutcomes(stmt.Values, len(stmt.Names))
			for i, name := range stmt.Names {
				c.storeOutcome(name, outcomes[i])
			}
		}
		return true
	})
}

func (c flakeChecks) walkCommunications(body *ast.BlockStmt, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	merged := make(map[types.Object]outcomePolarity)
	for _, clause := range body.List {
		branch := c
		branch.outcomes = maps.Clone(c.outcomes)
		branch.walk(clause, visit)
		mergeOutcomes(merged, branch.outcomes)
	}
	// Only a selected clause can reach the statement after the select.
	clear(c.outcomes)
	mergeOutcomes(c.outcomes, merged)
}

func (c flakeChecks) walkCases(body *ast.BlockStmt, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	merged := make(map[types.Object]outcomePolarity)
	var fallingThrough map[types.Object]outcomePolarity
	hasDefault := false
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		hasDefault = hasDefault || clause.List == nil
		branch := c
		branch.outcomes = maps.Clone(c.outcomes)
		for _, expr := range clause.List {
			branch.walk(expr, visit)
		}
		// Fallthrough enters the next body without evaluating the case expressions.
		mergeOutcomes(branch.outcomes, fallingThrough)
		for _, stmt := range clause.Body {
			branch.walk(stmt, visit)
		}
		if len(clause.Body) > 0 && isFallthrough(clause.Body[len(clause.Body)-1]) {
			fallingThrough = branch.outcomes
		} else {
			fallingThrough = nil
			mergeOutcomes(merged, branch.outcomes)
		}
	}
	if hasDefault {
		clear(c.outcomes)
	}
	mergeOutcomes(c.outcomes, merged)
}

func mergeOutcomes(dst, src map[types.Object]outcomePolarity) {
	for object, polarity := range src {
		dst[object] |= polarity
	}
}

// assignmentOutcomes reads the incoming state before any target is updated.
func (c flakeChecks) assignmentOutcomes(values []ast.Expr, targetCount int) []outcomePolarity {
	outcomes := make([]outcomePolarity, targetCount)
	if len(values) != targetCount {
		return outcomes
	}
	for i, value := range values {
		if !booleanResult(c.info, value) {
			continue
		}
		if c.failureCondition(value, true, nil) {
			outcomes[i] |= failsWhenTrue
		}
		if c.failureCondition(value, false, nil) {
			outcomes[i] |= failsWhenFalse
		}
	}
	return outcomes
}

func (c flakeChecks) storeOutcome(name *ast.Ident, polarity outcomePolarity) {
	if name.Name == "_" {
		return
	}
	object := c.info.ObjectOf(name)
	if polarity == 0 {
		delete(c.outcomes, object)
	} else {
		c.outcomes[object] = polarity
	}
}

func (c flakeChecks) retriesLoop(body *ast.BlockStmt) bool {
	// The main traversal still needs the state from before entering the loop.
	c.outcomes = maps.Clone(c.outcomes)
	found := false
	c.walk(body, func(c flakeChecks, node, parent ast.Node) bool {
		if found {
			return false
		}
		switch stmt := node.(type) {
		case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		case *ast.IfStmt:
			for _, branch := range c.outcomeBranches(stmt, false, body) {
				if exitsAttempt(lastStatement(branch)) {
					found = true
					return false
				}
			}
			found = c.continuesUntilSuccess(stmt, body, parent)
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

func (c flakeChecks) outcomeBranches(stmt *ast.IfStmt, failure bool, attempt ast.Node) [2]ast.Stmt {
	var branches [2]ast.Stmt
	if c.failureCondition(stmt.Cond, failure, attempt) {
		branches[0] = stmt.Body
	}
	if stmt.Else != nil && c.failureCondition(stmt.Cond, !failure, attempt) {
		branches[1] = stmt.Else
	}
	return branches
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
		if polarity := c.outcomes[c.info.ObjectOf(expr)]; polarity != 0 && attemptResult(c.info, expr, attempt) {
			if failure {
				return polarity&failsWhenTrue != 0
			}
			return polarity&failsWhenFalse != 0
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

func lastStatement(branch ast.Stmt) ast.Stmt {
	block, ok := branch.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return nil
	}
	return block.List[len(block.List)-1]
}

func exitsAttempt(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.BREAK
	}
	return false
}

func (c flakeChecks) continuesUntilSuccess(stmt *ast.IfStmt, body *ast.BlockStmt, parent ast.Node) bool {
	if stmt.Else != nil || !c.failureCondition(stmt.Cond, true, body) {
		return false
	}
	branch, ok := lastStatement(stmt.Body).(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE {
		return false
	}
	// A continue on failure is retry evidence only if success leaves the loop.
	block, ok := parent.(*ast.BlockStmt)
	if !ok {
		return false
	}
	next := slices.Index(block.List, ast.Stmt(stmt)) + 1
	for _, sibling := range block.List[next:] {
		if exitsAttempt(sibling) {
			return true
		}
		if _, jump := sibling.(*ast.BranchStmt); jump {
			return false
		}
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
