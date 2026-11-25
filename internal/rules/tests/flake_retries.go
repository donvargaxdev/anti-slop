package tests

import (
	"go/ast"
	"go/token"
	"go/types"
)

type retryScan struct {
	body        *ast.BlockStmt
	target      *branchTarget[flakeFlow]
	successExit bool
}

func (c flakeChecks) retriesLoop(body *ast.BlockStmt, post ast.Stmt, parent ast.Node) bool {
	var label types.Object
	if labeled, ok := parent.(*ast.LabeledStmt); ok {
		label = c.info.Defs[labeled.Label]
	}
	// Only the attempt's loop and targets inside it are relevant to the back edge.
	c.target = nil
	attempt := c.withTarget(label, true)
	attempt.retry = &retryScan{body: body, target: attempt.target}
	visit := func(c flakeChecks, node, _ ast.Node) bool {
		switch node.(type) {
		case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt:
			return false
		case *ast.ReturnStmt, *ast.BranchStmt:
			if c.successCase && c.exitsAttempt(node) {
				c.retry.successExit = true
			}
		}
		return true
	}
	attempt.walk(body, visit)
	attempt.flakeFlow = mergeFlakeFlows(attempt.flakeFlow, attempt.target.continues)
	attempt.walk(post, visit)
	return attempt.retry.successExit && attempt.outcomes != nil && attempt.retryFailure
}

func (c flakeChecks) trackRetryBranches(stmt *ast.IfStmt, parent ast.Node, then, otherwise *flakeChecks) {
	// A success-only exit leaves the opposite branch available to failed checks.
	if c.successCondition(stmt.Cond, true, c.retry.body) && c.exitsAttempt(lastStatement(stmt.Body)) {
		then.successCase = true
		otherwise.retryFailure = true
	}
	if c.successCondition(stmt.Cond, false, c.retry.body) && c.exitsAttempt(lastStatement(stmt.Else)) {
		otherwise.successCase = true
		then.retryFailure = true
	}
	if c.failureCondition(stmt.Cond, true, c.retry.body) && c.continuesUntilSuccess(stmt, parent) {
		c.retry.successExit = true
		then.retryFailure = true
	}
}

func lastStatement(branch ast.Stmt) ast.Stmt {
	block, ok := branch.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return nil
	}
	return block.List[len(block.List)-1]
}

func (c flakeChecks) exitsAttempt(stmt ast.Node) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		if stmt.Tok == token.BREAK {
			target := c.target.destination(c.info, stmt)
			return target == nil || target == c.retry.target
		}
	}
	return false
}

func (c flakeChecks) continuesUntilSuccess(stmt *ast.IfStmt, parent ast.Node) bool {
	if stmt.Else != nil {
		return false
	}
	branch, ok := lastStatement(stmt.Body).(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || c.target.destination(c.info, branch) != c.retry.target {
		return false
	}
	// A continue on failure is retry evidence only if success leaves the loop.
	block, ok := parent.(*ast.BlockStmt)
	if !ok {
		return false
	}
	for _, sibling := range block.List {
		if sibling.Pos() < stmt.End() {
			continue
		}
		sibling = unlabel(sibling)
		if c.exitsAttempt(sibling) {
			return true
		}
		if _, jump := sibling.(*ast.BranchStmt); jump {
			return false
		}
	}
	return false
}
