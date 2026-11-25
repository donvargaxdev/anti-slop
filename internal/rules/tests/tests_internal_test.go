package tests

import (
	"go/ast"
	"go/constant"
	"go/types"
	"testing"
)

func TestForBounds(t *testing.T) {
	alwaysFalse := ast.NewIdent("false")
	alwaysTrue := ast.NewIdent("true")
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		alwaysFalse: {Value: constant.MakeBool(false)},
		alwaysTrue:  {Value: constant.MakeBool(true)},
	}}
	for _, tc := range []struct {
		name string
		cond ast.Expr
		want loopBounds
	}{
		{"constant false", alwaysFalse, loopBounds{maxIterations: 0, maySkip: true}},
		{"constant true", alwaysTrue, loopBounds{maxIterations: unknownIterations}},
		{"nonconstant", ast.NewIdent("ready"), loopBounds{maxIterations: unknownIterations, maySkip: true}},
		{"unconditional", nil, loopBounds{maxIterations: unknownIterations}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := forBounds(info, tc.cond); got != tc.want {
				t.Errorf("forBounds = %+v, want %+v", got, tc.want)
			}
		})
	}
}
