package funclen

import (
	"go/ast"
	"go/token"
	"maps"

	"golang.org/x/tools/go/cfg"
)

type tables map[*ast.CompositeLit]struct{}

// REACHING CASE TABLES
// Event                        State after the event
// Literal assignment           Replace the target with the new literal
// Alias or whole-slice copy    Replace the target with the source's set
// Other write or range target  Clear the target
// Control flow joins           Union the sets from reachable predecessors
// Range expression             Record its literals and preserve the state
// Function literal             Pass the state to the callback and preserve the state
type reachingTables map[*ast.Ident]tables

type tableAnalysis struct {
	tableSyntax
	used     tables
	captures map[*ast.BlockStmt]reachingTables
}

func caseTables(file *ast.File) tables {
	analysis := tableAnalysis{
		tableSyntax: findTableSyntax(file),
		used:        make(tables), captures: make(map[*ast.BlockStmt]reachingTables),
	}
	if len(analysis.ranges) == 0 {
		return analysis.used
	}
	for node := range ast.Preorder(file) {
		body := functionBody(node)
		if body != nil {
			analysis.walk(body)
		}
	}
	return analysis.used
}

func (analysis tableAnalysis) walk(body *ast.BlockStmt) {
	graph := cfg.New(body, func(*ast.CallExpr) bool { return true })
	entry := graph.Blocks[0]
	inputs := make([]reachingTables, len(graph.Blocks))
	inputs[entry.Index] = make(reachingTables)
	mergeTables(inputs[entry.Index], analysis.captures[body])
	delete(analysis.captures, body)
	pending := []*cfg.Block{entry}
	for len(pending) > 0 {
		block := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		locals := maps.Clone(inputs[block.Index])
		analysis.enterBlock(block, locals)
		for _, node := range block.Nodes {
			analysis.visit(node, locals)
		}
		for _, next := range block.Succs {
			firstVisit := inputs[next.Index] == nil
			if firstVisit {
				inputs[next.Index] = make(reachingTables)
			}
			changed := mergeTables(inputs[next.Index], locals)
			if firstVisit || changed {
				pending = append(pending, next)
			}
		}
	}
}

func (analysis tableAnalysis) enterBlock(block *cfg.Block, locals reachingTables) {
	switch block.Kind {
	case cfg.KindRangeBody:
		if loop, ok := block.Stmt.(*ast.RangeStmt); ok {
			analysis.write(locals, variable(loop.Key), nil)
			analysis.write(locals, variable(loop.Value), nil)
		}
	case cfg.KindSelectCaseBody:
		if clause, ok := block.Stmt.(*ast.CommClause); ok {
			if assignment, ok := clause.Comm.(*ast.AssignStmt); ok {
				analysis.assign(locals, assignment.Lhs, nil)
			}
		}
	}
}

func (analysis tableAnalysis) visit(node ast.Node, locals reachingTables) {
	if expr, ok := node.(ast.Expr); ok {
		if _, ranges := analysis.ranges[expr]; ranges {
			maps.Copy(analysis.used, analysis.origins(expr, locals))
		}
	}
	ast.PreorderStack(node, nil, func(child ast.Node, _ []ast.Node) bool {
		literal, ok := child.(*ast.FuncLit)
		if !ok {
			return true
		}
		if analysis.captures[literal.Body] == nil {
			analysis.captures[literal.Body] = make(reachingTables)
		}
		mergeTables(analysis.captures[literal.Body], locals)
		return false
	})
	switch stmt := node.(type) {
	case *ast.AssignStmt:
		_, selectComm := analysis.selectComms[stmt]
		if !selectComm && (stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE) {
			analysis.assign(locals, stmt.Lhs, stmt.Rhs)
		}
	case *ast.ValueSpec:
		for index, id := range stmt.Names {
			analysis.write(locals, id, analysis.source(stmt.Values, index, locals))
		}
	}
}

func (analysis tableAnalysis) assign(locals reachingTables, targets, values []ast.Expr) {
	sources := make([]tables, len(targets))
	for i := range targets {
		sources[i] = analysis.source(values, i, locals)
	}
	for i, target := range targets {
		analysis.write(locals, variable(target), sources[i])
	}
}

func (analysis tableAnalysis) source(values []ast.Expr, index int, locals reachingTables) tables {
	if index < len(values) {
		return analysis.origins(values[index], locals)
	}
	return nil
}

func (analysis tableAnalysis) write(locals reachingTables, id *ast.Ident, origins tables) {
	if target := analysis.writes[id]; target != nil {
		if len(origins) == 0 {
			delete(locals, target)
		} else {
			locals[target] = origins
		}
	}
}

func (analysis tableAnalysis) origins(expr ast.Expr, locals reachingTables) tables {
	base := ast.Unparen(expr)
	for {
		slice, ok := base.(*ast.SliceExpr)
		if !ok {
			break
		}
		if slice.Low != nil || slice.High != nil || slice.Slice3 {
			return nil
		}
		base = ast.Unparen(slice.X)
	}
	switch value := base.(type) {
	case *ast.CompositeLit:
		return tables{value: {}}
	case *ast.Ident:
		return locals[analysis.names.at(value.Name, value.Pos()).name]
	}
	return nil
}

func mergeTables(destination, source reachingTables) bool {
	changed := false
	for name, incoming := range source {
		prior := destination[name]
		for table := range incoming {
			if _, known := prior[table]; known {
				continue
			}
			merged := maps.Clone(incoming)
			maps.Copy(merged, prior)
			destination[name] = merged
			changed = true
			break
		}
	}
	return changed
}
