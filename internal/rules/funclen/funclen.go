// Package funclen reports Go function bodies longer than 70 physical lines.
package funclen

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	name  = "funclen"
	limit = 70
)

// Analyzer checks function, method and function literal bodies.
var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "report function bodies longer than 70 lines",
	Run: func(pass *analysis.Pass) (any, error) {
		reportFiles(pass.Fset, pass.Files, pass.Report)
		return nil, nil
	},
}

// reportFiles checks physical lines so line directives cannot change the count.
func reportFiles(fset *token.FileSet, files []*ast.File, report func(analysis.Diagnostic)) {
	for _, file := range files {
		if ast.IsGenerated(file) {
			continue
		}
		testFile := strings.HasSuffix(fset.File(file.Pos()).Name(), "_test.go")
		for node := range ast.Preorder(file) {
			var body *ast.BlockStmt
			switch function := node.(type) {
			case *ast.FuncDecl:
				body = function.Body
			case *ast.FuncLit:
				body = function.Body
			}
			if body == nil {
				continue
			}
			lines := interiorLines(fset, body.Lbrace, body.Rbrace)
			if testFile && lines > limit {
				lines -= testCaseLines(fset, body)
			}
			if lines > limit {
				report(analysis.Diagnostic{
					Pos: node.Pos(), Category: name,
					Message: fmt.Sprintf("function body is %d lines; limit is %d", lines, limit),
				})
			}
		}
	}
}

func interiorLines(fset *token.FileSet, open, close token.Pos) int {
	return max(0, fset.PositionFor(close, false).Line-fset.PositionFor(open, false).Line-1)
}

// testCaseLines counts case data in the body and its nested blocks. The loops
// and their assertions still count toward the limit.
func testCaseLines(fset *token.FileSet, body *ast.BlockStmt) int {
	lines := make(map[int]struct{})
	for table := range caseTables(body) {
		open := fset.PositionFor(table.Lbrace, false).Line
		closeLine := fset.PositionFor(table.Rbrace, false).Line
		for line := open + 1; line < closeLine; line++ {
			lines[line] = struct{}{}
		}
	}
	return len(lines)
}

// Bindings are immutable, so branches can share their incoming history.
// An empty table set still shadows an earlier literal while in scope.
type caseBinding struct {
	start, end token.Pos
	tables     map[*ast.CompositeLit]struct{}
	previous   *caseBinding
}

type caseBindings map[string]*caseBinding

// caseTables finds inline tables and tables introduced within the body.
func caseTables(body *ast.BlockStmt) map[*ast.CompositeLit]struct{} {
	tables := make(map[*ast.CompositeLit]struct{})
	make(caseBindings).collect(body, nil, tables)
	return tables
}

func (locals caseBindings) collect(root ast.Node, stack []ast.Node, tables map[*ast.CompositeLit]struct{}) {
	ast.PreorderStack(root, stack, func(node ast.Node, stack []ast.Node) bool {
		switch node := node.(type) {
		case *ast.IfStmt:
			locals.collectIf(node, append(stack, node), tables)
			return false
		case *ast.SwitchStmt:
			locals.collectSwitch(node.Init, node.Tag, node.Body, append(stack, node), tables)
			return false
		case *ast.TypeSwitchStmt:
			locals.collectSwitch(node.Init, node.Assign, node.Body, append(stack, node), tables)
			return false
		case *ast.AssignStmt:
			for i, target := range node.Lhs {
				if id, ok := target.(*ast.Ident); ok {
					end := caseScopeEnd(stack)
					if node.Tok == token.ASSIGN {
						if binding := locals.at(id.Name, id.Pos()); binding != nil {
							end = assignmentScopeEnd(stack, binding.end)
						}
					}
					locals.bind(id, node.Rhs, i, node.End(), end)
				}
			}
		case *ast.ValueSpec:
			for i, id := range node.Names {
				locals.bind(id, node.Values, i, node.End(), caseScopeEnd(stack))
			}
		case *ast.FuncLit:
			locals.bindFields(node.Type.Params, node.Body)
			locals.bindFields(node.Type.Results, node.Body)
		case *ast.RangeStmt:
			locals.recordRange(node.X, tables)
			if node.Tok == token.DEFINE {
				for _, target := range []ast.Expr{node.Key, node.Value} {
					if id, ok := target.(*ast.Ident); ok {
						locals.bind(id, nil, 0, node.Body.Pos(), node.Body.End())
					}
				}
			}
		}
		return true
	})
}

func (locals caseBindings) collectIf(node *ast.IfStmt, stack []ast.Node, tables map[*ast.CompositeLit]struct{}) {
	if node.Init != nil {
		locals.collect(node.Init, stack, tables)
	}
	locals.collect(node.Cond, stack, tables)
	then := maps.Clone(locals)
	then.collect(node.Body, stack, tables)
	otherwise := maps.Clone(locals)
	if node.Else != nil {
		otherwise.collect(node.Else, stack, tables)
	}
	locals.joinBranches(node.End(), then, otherwise)
}

func (locals caseBindings) collectSwitch(init ast.Stmt, guard ast.Node, body *ast.BlockStmt, stack []ast.Node, tables map[*ast.CompositeLit]struct{}) {
	if init != nil {
		locals.collect(init, stack, tables)
	}
	if guard != nil {
		locals.collect(guard, stack, tables)
	}

	var exits []caseBindings
	var falling caseBindings
	hasDefault := false
	stack = append(stack, body)
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		clauseStack := append(stack, clause)
		hasDefault = hasDefault || clause.List == nil
		for _, expr := range clause.List {
			locals.collect(expr, clauseStack, tables)
		}
		branch := maps.Clone(locals)
		if falling != nil {
			branch.joinBranches(clause.Pos(), locals, falling)
		}
		for _, stmt := range clause.Body {
			branch.collect(stmt, clauseStack, tables)
		}
		falling = nil
		if fallsThrough(clause) {
			falling = branch
		} else {
			exits = append(exits, branch)
		}
	}
	if !hasDefault {
		exits = append(exits, locals)
	}
	locals.joinBranches(body.End(), exits...)
}

func fallsThrough(clause *ast.CaseClause) bool {
	for _, stmt := range slices.Backward(clause.Body) {
		if _, empty := stmt.(*ast.EmptyStmt); empty {
			continue
		}
		for label, ok := stmt.(*ast.LabeledStmt); ok; label, ok = stmt.(*ast.LabeledStmt) {
			stmt = label.Stmt
		}
		last, ok := stmt.(*ast.BranchStmt)
		return ok && last.Tok == token.FALLTHROUGH
	}
	return false
}

// Looking up bindings at the join drops declarations scoped to a branch.
func (locals caseBindings) joinBranches(end token.Pos, branches ...caseBindings) {
	for name, previous := range locals {
		incoming := locals.at(name, end)
		if incoming == nil {
			continue
		}
		var tables map[*ast.CompositeLit]struct{}
		for _, branch := range branches {
			binding := branch.at(name, end)
			if binding != incoming {
				tables = make(map[*ast.CompositeLit]struct{})
				break
			}
		}
		if tables == nil {
			continue
		}
		for _, branch := range branches {
			maps.Copy(tables, branch.at(name, end).tables)
		}
		locals[name] = &caseBinding{end, incoming.end, tables, previous}
	}
}

func caseScopeEnd(stack []ast.Node) token.Pos {
	for i := len(stack) - 1; i > 0; i-- {
		switch stack[i].(type) {
		case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.CaseClause, *ast.CommClause:
			return stack[i].End()
		}
	}
	return stack[0].End()
}

// A write in a function literal can affect later ranges in that literal.
// Its source position alone cannot tell us when it runs in the enclosing body.
func assignmentScopeEnd(stack []ast.Node, end token.Pos) token.Pos {
	for i := len(stack) - 1; i > 0; i-- {
		if function, ok := stack[i].(*ast.FuncLit); ok {
			return min(end, function.Body.End())
		}
	}
	return end
}

func (locals caseBindings) bind(id *ast.Ident, values []ast.Expr, index int, start, end token.Pos) {
	var tables map[*ast.CompositeLit]struct{}
	if index < len(values) {
		switch value := ast.Unparen(values[index]).(type) {
		case *ast.CompositeLit:
			tables = map[*ast.CompositeLit]struct{}{value: {}}
		case *ast.Ident:
			if binding := locals.at(value.Name, value.Pos()); binding != nil {
				tables = binding.tables
			}
		}
	}
	locals[id.Name] = &caseBinding{start, end, tables, locals[id.Name]}
}

func (locals caseBindings) bindFields(fields *ast.FieldList, body *ast.BlockStmt) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, id := range field.Names {
			locals.bind(id, nil, 0, body.Pos(), body.End())
		}
	}
}

func (locals caseBindings) recordRange(expr ast.Expr, tables map[*ast.CompositeLit]struct{}) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CompositeLit:
		tables[expr] = struct{}{}
	case *ast.Ident:
		if binding := locals.at(expr.Name, expr.Pos()); binding != nil {
			maps.Copy(tables, binding.tables)
		}
	}
}

func (locals caseBindings) at(name string, pos token.Pos) *caseBinding {
	for binding := locals[name]; binding != nil; binding = binding.previous {
		if binding.start <= pos && pos < binding.end {
			return binding
		}
	}
	return nil
}
