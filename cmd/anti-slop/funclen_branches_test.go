package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCaseTableSyntax(t *testing.T) {
	const table = "[]int{\n0,\n1,\n}"
	const data = "[]int{\n0,\n// comment between cases\n\n1,\n}"
	longCases := "[]int{\n" + strings.Repeat("0,\n", 71) + "}"
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"inline boundary", strings.Repeat("_ = n\n", 65) +
			"for _, v := range " + table + " {\n_ = v\n_ = n\n}\n", 0},
		{"inline loop body exceeds limit", strings.Repeat("_ = n\n", 66) +
			"for _, v := range " + table + " {\n_ = v\n_ = n\n}\n", 71},
		{"adjacent declaration boundary", strings.Repeat("_ = n\n", 65) +
			"cases := " + table + "\nfor _, v := range cases {\n_ = v\n}\n", 0},
		{"adjacent declaration exceeds limit", strings.Repeat("_ = n\n", 66) +
			"cases := " + table + "\nfor _, v := range cases {\n_ = v\n}\n", 71},
		{"comment and blank lines between cases count", strings.Repeat("_ = n\n", 65) +
			"for _, v := range " + data + " {\n_ = v\n}\n", 71},
		{"opening brace line counts even with a case", strings.Repeat("_ = n\n", 67) +
			"for range []int{0,\n1,\n} {\n_ = n\n}\n", 71},
		{"closing brace line counts even with a case", strings.Repeat("_ = n\n", 67) +
			"for range []int{\n0,\n1} {\n_ = n\n}\n", 71},
		{"all lines of a multiline case are exempt", strings.Repeat("_ = n\n", 66) +
			"for range [][]int{\n{\n0,\n1,\n},\n} {\n_ = n\n}\n", 0},
		{"alias counts", "cases := " + longCases + "\nalias := cases\nfor range alias {}\n", 75},
		{"slice expression counts", "cases := " + longCases + "\nfor range cases[1:] {}\n", 74},
		{"full slice expression counts", "cases := " + longCases + "\nfor range cases[:] {}\n", 74},
		{"intervening setup counts", "cases := " + longCases + "\n_ = n\nfor range cases {}\n", 75},
		{"declaration outside the loop block counts", "cases := " + longCases +
			"\nif n > 0 {\nfor range cases {}\n}\n", 76},
		{"assignment before a range counts", "var cases []int\ncases = " + longCases +
			"\nfor range cases {}\n", 75},
		{"multiple declared names count", "cases, other := " + longCases +
			", n\nfor range cases {}\n_ = other\n", 75},
		{"range of another variable counts", "other := []int{n}\ncases := " + longCases +
			"\nfor range other {}\n_ = cases\n", 76},
		{"assignment through a loop with an early exit counts", "var cases []int\n" +
			"for range n {\nif n > 1 { break }\ncases = " + longCases + "\n}\nfor range cases {}\n", 78},
		{"declaration before an early exit counts", "cases := " + longCases +
			"\nfor {\nif n > 0 { break }\ncases = nil\n}\nfor range cases {}\n", 78},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const header = "package example\nfunc test(n int) {\n"
			var want string
			if tc.want != 0 {
				want = fmt.Sprintf("cases_test.go:2:1: funclen: function body is %d lines; limit is 70\n", tc.want)
			}
			assertFunctionLengthOutput(t, "cases_test.go", header+tc.body+"}\n", want)
		})
	}
}

func TestCaseTableAssignmentsToParametersAndResultsCount(t *testing.T) {
	const header = "package example\nfunc helper(cases []int, flag bool) (result []int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	source := header + "if flag {\ncases = " + literal + "}\nfor range cases {}\n" +
		"if flag {\nresult = " + literal + "}\nfor range result {}\nreturn\n}\n"
	const want = "cases_test.go:2:1: funclen: function body is 153 lines; limit is 70\n"
	assertFunctionLengthOutput(t, "cases_test.go", source, want)
}
