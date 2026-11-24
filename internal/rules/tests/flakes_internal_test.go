package tests

import (
	"go/ast"
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
