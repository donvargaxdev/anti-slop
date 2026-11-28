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
		{"local alias", "cases := " + longCases + "\nalias := cases\nfor range alias {}\n", 0},
		{"alias of another variable counts", "other := []int{n}\ncases := " + longCases +
			"\nalias := other\nfor range alias {}\n_ = cases\n", 77},
		{"slice expression counts", "cases := " + longCases + "\nfor range cases[1:] {}\n", 74},
		{"full slice expression", "cases := " + longCases + "\nfor range cases[:] {}\n", 0},
		{"bounded slice expression counts", "cases := " + longCases + "\nfor range cases[:1] {}\n", 74},
		{"three-index slice expression counts", "cases := " + longCases + "\nfor range cases[:1:1] {}\n", 74},
		{"intervening setup", "cases := " + longCases + "\n_ = n\nfor range cases {}\n", 75},
		{"declaration outside the loop block", "cases := " + longCases +
			"\nif n > 0 {\nfor range cases {}\n}\n", 76},
		{"assignment before a range", "var cases []int\ncases = " + longCases +
			"\nfor range cases {}\n", 75},
		{"multiple declared names", "cases, other := " + longCases +
			", n\nfor range cases {}\n_ = other\n", 75},
		{"range of another variable counts", "other := []int{n}\ncases := " + longCases +
			"\nfor range other {}\n_ = cases\n", 76},
		{"assignment in a loop with an early exit", "var cases []int\n" +
			"for range n {\nif n > 1 { break }\ncases = " + longCases + "\n}\nfor range cases {}\n", 78},
		{"declaration preserved by an early exit", "cases := " + longCases +
			"\nfor {\nif n > 0 { break }\ncases = nil\n}\nfor range cases {}\n", 0},
		{"overwrite before an early exit counts", "cases := " + longCases +
			"\nfor {\ncases = nil\nif n > 0 { break }\n}\nfor range cases {}\n", 78},
		{"loop initializer overwrite counts", "cases := " + longCases +
			"\nfor cases = nil; ; {\nif n > 0 { break }\n}\nfor range cases {}\n", 77},
		{"guard initializer overwrite counts", "cases := " + longCases +
			"\nfor {\nif cases = nil; n > 0 { break }\n}\nfor range cases {}\n", 77},
		{"guard overwrite counts", "cases := " + longCases +
			"\nfor {\nif n > 0 { cases = nil; break }\n}\nfor range cases {}\n", 77},
		{"break to another loop counts", "outer: for {\ncases := " + longCases +
			"\nfor {\nif n > 0 { break outer }\n}\nfor range cases {}\n}\n", 79},
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

func TestLabeledCaseRangeCountsLabelAndAssertions(t *testing.T) {
	const header = "package example\nfunc test(n int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}"
	loop := "each:\nfor _, value := range " + literal + " {\nif value == n {\ncontinue each\n}\n}\n"
	t.Run("70 counted lines", func(t *testing.T) {
		source := header + strings.Repeat("_ = n\n", 63) + loop + "}\n"
		assertFunctionLengthOutput(t, "cases_test.go", source, "")
	})
	t.Run("71 counted lines", func(t *testing.T) {
		source := header + strings.Repeat("_ = n\n", 64) + loop + "}\n"
		const want = "cases_test.go:2:1: funclen: function body is 71 lines; limit is 70\n"
		assertFunctionLengthOutput(t, "cases_test.go", source, want)
	})
}

func TestNestedCaseTableSyntax(t *testing.T) {
	const header = "package example\nfunc test(n int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	for _, tc := range []struct {
		name   string
		prefix string
		suffix string
	}{
		{"callback", "_ = func() {\n", "}\n"},
		{"switch clause", "switch n {\ncase 0:\n", "}\n"},
		{"select clause", "select {\ndefault:\n", "}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const loop = "for _, value := range cases {\n_ = value\n}\n"
			source := header + tc.prefix + "cases := " + literal + loop + tc.suffix + "}\n"
			assertFunctionLengthOutput(t, "cases_test.go", source, "")
		})
	}
}
