package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

func (c flakeChecks) recordBooleanReturn(stmt *ast.ReturnStmt) {
	returns := c.returns
	possible := failsWhenTrue | failsWhenFalse
	polarity := c.outcomes[returns.result]
	if polarity&constantTrue != 0 {
		possible = failsWhenTrue
	} else if polarity&constantFalse != 0 {
		possible = failsWhenFalse
	}
	polarity = polarity.withGuardedAssignments()
	if len(stmt.Results) == 1 {
		expr := stmt.Results[0]
		polarity = c.booleanOutcome(expr)
		if value := c.info.Types[expr].Value; value != nil {
			possible = failsWhenFalse
			if constant.BoolVal(value) {
				possible = failsWhenTrue
			}
		}
	}
	returns.onlyShort = polarity == shortMode && (returns.flow.outcomes == nil || returns.onlyShort)
	if c.failureGuard {
		polarity = possible
	}
	returns.guarded |= possible & polarity
	returns.unguarded |= possible &^ polarity
}

func (r flakeReturns) booleanOutcome() outcomePolarity {
	// A return value implies failure only if every path producing it requires failure.
	if polarity := r.guarded &^ r.unguarded; polarity != 0 {
		return polarity
	}
	if r.onlyShort {
		return shortMode
	}
	return 0
}

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

func (c flakeChecks) retryBooleanCheck(value ast.Expr) bool {
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok {
		return false
	}
	// A callback or a method predicate may only be waiting for readiness.
	fn := calledFunc(c.info, call)
	return fn != nil && fn.Signature().Recv() == nil
}

func (c flakeChecks) failureCondition(condition ast.Expr, truth bool, attempt ast.Node) bool {
	return c.checkCondition(condition, truth, true, attempt)
}

func (c flakeChecks) successCondition(condition ast.Expr, truth bool, attempt ast.Node) bool {
	return c.checkCondition(condition, truth, false, attempt)
}

// checkCondition tests what this branch implies about a check's outcome.
// In a retry loop a result must come from inside the loop.
func (c flakeChecks) checkCondition(condition ast.Expr, truth, wantFailure bool, attempt ast.Node) bool {
	failure := truth == wantFailure
	switch expr := ast.Unparen(condition).(type) {
	case *ast.UnaryExpr:
		return expr.Op == token.NOT && c.checkCondition(expr.X, !truth, wantFailure, attempt)
	case *ast.BinaryExpr:
		return c.checkComparison(expr, truth, wantFailure, attempt)
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
		case c.isErrorsIsNil(expr):
			value := expr.Args[0]
			// A concrete error value is boxed into a non-nil interface, even if the value is nil.
			return !failure && types.IsInterface(c.info.TypeOf(value)) && c.errorComparedWithNil(value, expr.Args[1], attempt)
		case packageCall(c.info, expr, "errors", "Is", "As"):
			// Classifying an error only retries a check if the error is refreshed.
			return failure && (!c.retryHeader || attempt == nil || c.retryErrorOutcome(expr.Args[0]) != 0)
		case testingMethod(c.info, expr, "Run"):
			return !failure
		case attempt != nil || c.retry != nil:
			return !failure && c.retryBooleanCheck(expr) && booleanResult(c.info, expr) && !c.isShortMode(expr)
		}
	case *ast.Ident:
		polarity := c.outcomes[c.info.ObjectOf(expr)].withGuardedAssignments()
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

func (c flakeChecks) constantAssignment(value bool) outcomePolarity {
	if value {
		if c.failureGuard {
			return constantTrue | guardedTrue
		}
		return constantTrue
	}
	if c.failureGuard {
		return constantFalse | guardedFalse
	}
	return constantFalse
}

func joinOutcome(left, right outcomePolarity) outcomePolarity {
	// Keep definite facts only when both paths agree.
	joined := left & right & (shortMode | nilError | constantTrue | constantFalse)
	joined |= (left | right) & refreshedError
	// A value implies failure only if each incoming path either requires
	// failure for that value or can only produce the opposite value.
	trueEvidence := guardedTrue | failsWhenTrue
	if allowed := trueEvidence | constantFalse; left&allowed != 0 && right&allowed != 0 {
		joined |= (left | right) & trueEvidence
	}
	falseEvidence := guardedFalse | failsWhenFalse
	if allowed := falseEvidence | constantTrue; left&allowed != 0 && right&allowed != 0 {
		joined |= (left | right) & falseEvidence
	}
	return joined
}

func (polarity outcomePolarity) withGuardedAssignments() outcomePolarity {
	if polarity&guardedTrue != 0 {
		polarity |= failsWhenTrue
	}
	if polarity&guardedFalse != 0 {
		polarity |= failsWhenFalse
	}
	return polarity
}

func restrictBooleanOutcome(polarity, possible outcomePolarity) outcomePolarity {
	if possible == failsWhenTrue {
		return constantTrue | polarity&(failsWhenTrue|guardedTrue)
	}
	return constantFalse | polarity&(failsWhenFalse|guardedFalse)
}

func (c flakeChecks) checkComparison(expr *ast.BinaryExpr, truth, wantFailure bool, attempt ast.Node) bool {
	switch expr.Op {
	case token.LAND, token.LOR:
		// A true conjunction or false disjunction fixes both operands.
		either := (expr.Op == token.LAND) == truth
		left := c.checkCondition(expr.X, truth, wantFailure, attempt)
		right := c.checkCondition(expr.Y, truth, wantFailure, attempt)
		if either {
			return left || right
		}
		return left && right
	case token.EQL, token.NEQ:
		if operand, same := booleanComparison(c.info, expr); operand != nil {
			return c.checkCondition(operand, truth == same, wantFailure, attempt)
		}
		if (expr.Op == token.NEQ) != (truth == wantFailure) {
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

// mutableVariables finds variables that can be written after initialization.
func mutableVariables(info *types.Info, file *ast.File) map[types.Object]bool {
	mutable := make(map[types.Object]bool)
	mark := func(targets ...ast.Expr) {
		for _, target := range targets {
			if name, ok := ast.Unparen(target).(*ast.Ident); ok {
				if variable, ok := info.Uses[name].(*types.Var); ok {
					mutable[variable] = true
				}
			}
		}
	}
	for node := range ast.Preorder(file) {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			mark(stmt.Lhs...)
		case *ast.RangeStmt:
			mark(stmt.Key, stmt.Value)
		case *ast.UnaryExpr:
			if stmt.Op == token.AND {
				mark(stmt.X)
			}
		}
	}
	return mutable
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
