package funclen

import (
	"go/ast"
	"go/token"
)

type declaration struct {
	name  *ast.Ident
	scope ast.Node
	start token.Pos
}

type lexicalNames map[string][]declaration

func (names lexicalNames) at(name string, pos token.Pos) declaration {
	var visible declaration
	for _, candidate := range names[name] {
		if candidate.start <= pos && pos < candidate.scope.End() && candidate.start >= visible.start {
			visible = candidate
		}
	}
	return visible
}

func (names lexicalNames) declare(id *ast.Ident, scope ast.Node, start token.Pos) {
	if id.Name != "_" {
		names[id.Name] = append(names[id.Name], declaration{id, scope, start})
	}
}

type tableSyntax struct {
	names       lexicalNames
	writes      map[*ast.Ident]*ast.Ident
	ranges      map[ast.Expr]struct{}
	selectComms map[*ast.AssignStmt]struct{}
}

func findTableSyntax(file *ast.File) tableSyntax {
	syntax := tableSyntax{
		names: make(lexicalNames), writes: make(map[*ast.Ident]*ast.Ident),
		ranges: make(map[ast.Expr]struct{}), selectComms: make(map[*ast.AssignStmt]struct{}),
	}
	ast.PreorderStack(file, nil, func(node ast.Node, parents []ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			syntax.bindFields(node.Body, node.Recv, node.Type.Params, node.Type.Results)
		case *ast.FuncLit:
			syntax.bindFields(node.Body, node.Type.Params, node.Type.Results)
		case *ast.AssignStmt:
			syntax.bindAssignment(node, parents)
		case *ast.ValueSpec:
			for _, id := range node.Names {
				syntax.names.declare(id, lexicalScope(parents), node.End())
				syntax.writes[id] = id
			}
		case *ast.TypeSpec:
			syntax.names.declare(node.Name, lexicalScope(parents), node.Pos())
		case *ast.RangeStmt:
			syntax.ranges[node.X] = struct{}{}
			syntax.bindRangeTargets(node)
		}
		return true
	})
	return syntax
}

func (syntax tableSyntax) bindFields(body *ast.BlockStmt, lists ...*ast.FieldList) {
	if body == nil {
		return
	}
	for _, list := range lists {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			for _, id := range field.Names {
				syntax.names.declare(id, body, body.Pos())
			}
		}
	}
}

func (syntax tableSyntax) bindAssignment(stmt *ast.AssignStmt, parents []ast.Node) {
	if clause, ok := parents[len(parents)-1].(*ast.CommClause); ok && clause.Comm == stmt {
		syntax.selectComms[stmt] = struct{}{}
	}
	scope := lexicalScope(parents)
	for _, target := range stmt.Lhs {
		id := variable(target)
		if id == nil {
			continue
		}
		prior := syntax.names.at(id.Name, id.Pos())
		if stmt.Tok == token.DEFINE && prior.scope != scope {
			syntax.names.declare(id, scope, stmt.End())
			syntax.writes[id] = id
		} else {
			syntax.writes[id] = prior.name
		}
	}
}

func (syntax tableSyntax) bindRangeTargets(loop *ast.RangeStmt) {
	for _, target := range []ast.Expr{loop.Key, loop.Value} {
		id := variable(target)
		if id == nil {
			continue
		}
		if loop.Tok == token.DEFINE {
			syntax.names.declare(id, loop, loop.Body.Pos())
			syntax.writes[id] = id
		} else {
			syntax.writes[id] = syntax.names.at(id.Name, id.Pos()).name
		}
	}
}

func lexicalScope(parents []ast.Node) ast.Node {
	for i := len(parents) - 1; i >= 0; i-- {
		switch parents[i].(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause, *ast.IfStmt,
			*ast.ForStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			return parents[i]
		}
	}
	return parents[0]
}

func variable(expr ast.Expr) *ast.Ident {
	id, _ := ast.Unparen(expr).(*ast.Ident)
	if id != nil && id.Name == "_" {
		return nil
	}
	return id
}
