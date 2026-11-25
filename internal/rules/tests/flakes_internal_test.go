package tests

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"
)

func TestBooleanResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  types.Type
		want bool
	}{
		{"boolean", types.Typ[types.Bool], true},
		{"integer", types.Typ[types.Int], false},
		{"unresolved", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr := ast.NewIdent("result")
			info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{expr: {Type: tc.typ}}}
			if got := booleanResult(info, expr); got != tc.want {
				t.Errorf("booleanResult(%v) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}

func TestAttemptResultExcludesEndPosition(t *testing.T) {
	attempt := &ast.BlockStmt{Lbrace: 10, Rbrace: 20}
	// Construct positions directly to test the exclusive end of the AST span.
	for _, tc := range []struct {
		name string
		pos  token.Pos
		want bool
	}{
		{"inside", attempt.Pos() + 1, true},
		{"end", attempt.End(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr := ast.NewIdent("result")
			object := types.NewVar(tc.pos, nil, expr.Name, types.Typ[types.Bool])
			checks := flakeChecks{info: &types.Info{Uses: map[*ast.Ident]types.Object{expr: object}}}
			if got := checks.attemptResult(expr, attempt); got != tc.want {
				t.Errorf("attemptResult for declaration at %d = %v, want %v", tc.pos, got, tc.want)
			}
		})
	}
}
