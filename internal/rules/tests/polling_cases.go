package tests

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (scan *pollingScan) recordSwitch(body *ast.BlockStmt, label types.Object, tagReadsClock bool) {
	entry := scan.withTarget(label, false)
	matches, unmatchedReadsClock := entry.matchPollingCases(body)
	scan.stopPath()
	var fallingThrough pollingFlow
	clockFallthrough := false
	guardedContinue, hasDefault := false, false
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		matched := matches[clause]
		if clause.List == nil {
			hasDefault = true
			matched = pollingCase{flow: entry.pollingFlow, readsClock: unmatchedReadsClock}
		}
		branch := *entry
		branch.pollingFlow = matched.flow
		clockGuard := tagReadsClock || matched.readsClock || clockFallthrough
		if clockGuard {
			scan.clockGuards[clause] = true
		}
		// Fallthrough enters the case body without testing the case expressions.
		branch.pollingFlow = mergePollingFlows(branch.pollingFlow, fallingThrough)
		fallsThrough := false
		before := scan.continueCount()
		for _, stmt := range clause.Body {
			branch.record(stmt)
			fallsThrough = fallsThrough || isFallthrough(stmt)
		}
		guardedContinue = guardedContinue || clockGuard && scan.continueCount() != before
		if fallsThrough {
			fallingThrough = branch.pollingFlow
			clockFallthrough = clockGuard
			continue
		}
		fallingThrough = pollingFlow{}
		clockFallthrough = false
		scan.pollingFlow = mergePollingFlows(scan.pollingFlow, branch.pollingFlow)
	}
	if !hasDefault {
		scan.pollingFlow = mergePollingFlows(scan.pollingFlow, entry.pollingFlow)
	}
	scan.pollingFlow = mergePollingFlows(scan.pollingFlow, entry.target.breaks)
	scan.markBypassed(guardedContinue)
}

type pollingCase struct {
	flow       pollingFlow
	readsClock bool
}

func (scan *pollingScan) matchPollingCases(body *ast.BlockStmt) (map[*ast.CaseClause]pollingCase, bool) {
	matches := make(map[*ast.CaseClause]pollingCase)
	unmatchedReadsClock := false
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		if clause.List == nil {
			continue
		}
		var matched pollingCase
		// Later expressions run only when earlier ones miss. Default uses the
		// state after all expressions miss, wherever its body appears.
		for _, expr := range clause.List {
			unmatchedReadsClock = scan.recordClockGuard(expr) || unmatchedReadsClock
			matched.flow = mergePollingFlows(matched.flow, scan.pollingFlow)
		}
		matched.readsClock = unmatchedReadsClock
		matches[clause] = matched
	}
	return matches, unmatchedReadsClock
}

func isFallthrough(stmt ast.Stmt) bool {
	branch, ok := unlabel(stmt).(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

func (scan *pollingScan) recordSelect(body *ast.BlockStmt, label types.Object) {
	entry := scan.withTarget(label, false)
	visitSelectOperands(body, func(expr ast.Expr) { entry.record(expr) })
	scan.stopPath()
	for _, stmt := range body.List {
		clause := stmt.(*ast.CommClause)
		branch := entry.fork()
		visitReceiveTargets(clause.Comm, func(expr ast.Expr) { branch.record(expr) }, func(name *ast.Ident) {
			branch.store(branch.info.ObjectOf(name), false)
		})
		for _, bodyStmt := range clause.Body {
			branch.record(bodyStmt)
		}
		scan.pollingFlow = mergePollingFlows(scan.pollingFlow, branch.pollingFlow)
	}
	scan.pollingFlow = mergePollingFlows(scan.pollingFlow, entry.target.breaks)
}
