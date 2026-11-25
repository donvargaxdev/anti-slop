package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

func (c flakeChecks) retriesInHeader(loop *ast.ForStmt, parent ast.Node) bool {
	if loop.Cond == nil {
		return false
	}
	var label types.Object
	if labeled, ok := parent.(*ast.LabeledStmt); ok {
		label = c.info.Defs[labeled.Label]
	}
	// Track only fresh results that reach the condition for the next attempt.
	c.flakeFlow = flakeFlow{outcomes: make(map[types.Object]outcomePolarity)}
	c.callOutcomes = make(map[*ast.CallExpr]outcomePolarity)
	c.retryHeader = true
	iteration := c.withTarget(label, true)
	visit := func(_ flakeChecks, _, _ ast.Node) bool { return true }
	iteration.walk(loop.Body, visit)
	iteration.flakeFlow = mergeFlakeFlows(iteration.flakeFlow, iteration.target.continues)
	iteration.walk(loop.Post, visit)
	if iteration.outcomes == nil {
		return false
	}
	return iteration.failureCondition(loop.Cond, true, loop.Body)
}

func (c flakeChecks) retryTupleOutcomes(values []ast.Expr, outcomes []outcomePolarity) {
	if len(values) != 1 {
		return
	}
	value := values[0]
	if !c.isCheckCall(value) {
		return
	}
	// A single call assigned to multiple targets has a tuple result.
	tuple := c.info.TypeOf(value).(*types.Tuple)
	for i := range outcomes {
		if isErrorResult(tuple.At(i).Type()) {
			outcomes[i] = refreshedError
		}
	}
}

func (c flakeChecks) retryErrorOutcome(value ast.Expr) outcomePolarity {
	if !isErrorResult(c.info.TypeOf(value)) {
		return 0
	}
	if name, ok := ast.Unparen(value).(*ast.Ident); ok {
		return c.outcomes[c.info.ObjectOf(name)] & refreshedError
	}
	if c.isCheckCall(value) {
		return refreshedError
	}
	return 0
}

func (c flakeChecks) isCheckCall(value ast.Expr) bool {
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	return ok && !c.info.Types[call.Fun].IsType()
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
		if polarity := c.callOutcomes[expr]; polarity != 0 {
			if failure {
				return polarity&failsWhenTrue != 0
			}
			return polarity&failsWhenFalse != 0
		}
		switch {
		case testingMethod(c.info, expr, "Failed"):
			return failure && (!c.retryHeader || attempt == nil)
		case packageCall(c.info, expr, "errors", "Is", "As"):
			// Classifying an error only retries a check if the error is refreshed.
			return failure && (!c.retryHeader || attempt == nil || c.retryErrorOutcome(expr.Args[0]) != 0)
		case testingMethod(c.info, expr, "Run"):
			return !failure
		case attempt != nil || c.retry != nil:
			if c.retryHeader {
				// A callback or a method predicate may only be waiting for readiness.
				fn := calledFunc(c.info, expr)
				if fn == nil || fn.Signature().Recv() != nil {
					return false
				}
			}
			return !failure && c.isCheckCall(expr) && booleanResult(c.info, expr) && !c.isShortMode(expr)
		}
	case *ast.Ident:
		polarity := c.outcomes[c.info.ObjectOf(expr)]
		if polarity == shortMode {
			return false
		}
		if c.attemptResult(expr, attempt) {
			if failure {
				return polarity&failsWhenTrue != 0
			}
			return polarity&failsWhenFalse != 0
		}
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
		if operand, same := booleanComparison(c.info, expr); operand != nil {
			return c.failureCondition(operand, failure == same, attempt)
		}
		if (expr.Op == token.NEQ) != failure {
			return false
		}
		return c.errorComparedWithNil(expr.X, expr.Y, attempt) ||
			c.errorComparedWithNil(expr.Y, expr.X, attempt)
	}
	return false
}

// booleanComparison returns the operand tested against a boolean constant and
// whether the comparison has the same truth value as that operand.
func booleanComparison(info *types.Info, expr *ast.BinaryExpr) (ast.Expr, bool) {
	if expr.Op != token.EQL && expr.Op != token.NEQ {
		return nil, false
	}
	operand := expr.X
	value := info.Types[expr.Y].Value
	if value == nil {
		operand = expr.Y
		value = info.Types[expr.X].Value
	}
	if value == nil || value.Kind() != constant.Bool {
		return nil, false
	}
	return operand, constant.BoolVal(value) == (expr.Op == token.EQL)
}

func (c flakeChecks) errorComparedWithNil(value, other ast.Expr, attempt ast.Node) bool {
	return c.isNil(other) && !c.knownNilError(value) && isErrorResult(c.info.TypeOf(value)) && c.attemptResult(value, attempt)
}

func isErrorResult(typ types.Type) bool {
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return typ != nil && types.Implements(typ, errorType)
}

func (c flakeChecks) attemptResult(value ast.Expr, attempt ast.Node) bool {
	if attempt == nil {
		return true
	}
	switch expr := ast.Unparen(value).(type) {
	case *ast.CallExpr:
		return c.isCheckCall(expr)
	case *ast.Ident:
		object := c.info.ObjectOf(expr)
		if c.retryHeader {
			return c.outcomes[object] != 0
		}
		return object != nil && (attempt.Pos() < object.Pos() && object.Pos() < attempt.End() ||
			contains(attempt, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if !ok {
					return false
				}
				for _, lhs := range assign.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && c.info.ObjectOf(ident) == object {
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

// Later operands can write a stored outcome through a function literal.
func (c flakeChecks) outcomeReassigned(object types.Object, later []ast.Expr) bool {
	for _, operand := range later {
		for node := range ast.Preorder(operand) {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				continue
			}
			for _, target := range assign.Lhs {
				if name, ok := ast.Unparen(target).(*ast.Ident); ok && c.info.Uses[name] == object {
					return true
				}
			}
		}
	}
	return false
}
