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
		{"slice expression counts", "cases := " + longCases + "\nfor range cases[1:] {}\n", 74},
		{"full slice expression", "cases := " + longCases + "\nfor range cases[:] {}\n", 0},
		{"intervening setup", "cases := " + longCases + "\n_ = n\nfor range cases {}\n", 0},
		{"declaration outside the loop block", "cases := " + longCases +
			"\nif n > 0 {\nfor range cases {}\n}\n", 0},
		{"assignment before a range", "var cases []int\ncases = " + longCases +
			"\nfor range cases {}\n", 0},
		{"multiple declared names", "cases, other := " + longCases +
			", n\nfor range cases {}\n_ = other\n", 0},
		{"range of another variable counts", "other := []int{n}\ncases := " + longCases +
			"\nfor range other {}\n_ = cases\n", 76},
		{"assignment that reaches the range despite a possible early exit", "var cases []int\n" +
			"for range n {\nif n > 1 { break }\ncases = " + longCases + "\n}\nfor range cases {}\n", 0},
		{"declaration preserved by an early exit", "cases := " + longCases +
			"\nfor {\nif n > 0 { break }\ncases = nil\n}\nfor range cases {}\n", 0},
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

func TestCaseTableAssignmentsToParametersAndResults(t *testing.T) {
	const header = "package example\nfunc helper(cases []int, flag bool) (result []int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	source := header + "if flag {\ncases = " + literal + "}\nfor range cases {}\n" +
		"if flag {\nresult = " + literal + "}\nfor range result {}\nreturn\n}\n"
	assertFunctionLengthOutput(t, "cases_test.go", source, "")
}

func TestLabeledCaseRangeCountsLabelAndAssertions(t *testing.T) {
	const header = "package example\nfunc test(n int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	const loop = "each:\nfor _, value := range cases {\nif value == n {\ncontinue each\n}\n}\n"
	t.Run("70 counted lines", func(t *testing.T) {
		source := header + strings.Repeat("_ = n\n", 62) + "cases := " + literal + loop + "}\n"
		assertFunctionLengthOutput(t, "cases_test.go", source, "")
	})
	t.Run("71 counted lines", func(t *testing.T) {
		source := header + strings.Repeat("_ = n\n", 63) + "cases := " + literal + loop + "}\n"
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

func TestCaseTableControlFlow(t *testing.T) {
	const header = "package example\nfunc example(n int, ch <-chan int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	const loop = "for _, value := range cases {\n_ = value\n}\n"
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"outer declaration used inside callback", "cases := " + literal + "_ = func() {\n" + loop + "}\n", 0},
		{"if else alternatives", "var cases []int\nif n == 0 {\ncases = " + literal +
			"} else {\ncases = " + literal + "}\n" + loop, 0},
		{"switch alternatives", "var cases []int\nswitch n {\ncase 0:\ncases = " + literal +
			"default:\ncases = " + literal + "}\n" + loop, 0},
		{"select alternatives", "var cases []int\nselect {\ncase <-ch:\ncases = " + literal +
			"default:\ncases = " + literal + "}\n" + loop, 0},
		{"optional loop preserves incoming table", "cases := " + literal +
			"for range n {\ncases = nil\n}\n" + loop, 0},
		{"previous iteration supplies table", "var cases []int\nfor range 2 {\n" +
			loop + "cases = " + literal + "}\n", 0},
		{"unreachable assignment after break counts", "var cases []int\nfor range n {\nbreak\ncases = " +
			literal + "}\n" + loop, 80},
		{"range assignment replaces existing table", "cases := " + literal +
			"for _, cases = range [][]int{nil, nil} {\n" + loop + "cases = " + literal + "}\n", 151},
		{"shadowing range uses the inner variable", "cases := " + literal +
			"if cases := []int{n}; n > 0 {\n" + loop + "}\n_ = cases\n", 79},
		{"unconditional overwrite kills table", "cases := " + literal + "cases = nil\n" + loop, 77},
		{"closure writes do not reach outer range", "cases := " + literal +
			"_ = func() { cases = nil }\n" + loop, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want string
			if tc.want != 0 {
				want = fmt.Sprintf("cases_test.go:2:1: funclen: function body is %d lines; limit is 70\n", tc.want)
			}
			assertFunctionLengthOutput(t, "cases_test.go", header+tc.body+"}\n", want)
		})
	}
}

func TestCaseTableSelectReceive(t *testing.T) {
	const header = "package example\nfunc example(ch <-chan []int) {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	const receive = "case cases = <-ch:\n"
	const loop = "for _, value := range cases {\n_ = value\n}\n"
	t.Run("receive replaces the table", func(t *testing.T) {
		source := header + "cases := " + literal + "select {\n" + receive + loop + "}\n}\n"
		const want = "cases_test.go:2:1: funclen: function body is 79 lines; limit is 70\n"
		assertFunctionLengthOutput(t, "cases_test.go", source, want)
	})
	t.Run("default preserves the table", func(t *testing.T) {
		source := header + "cases := " + literal + "select {\n" + receive + "default:\n" + loop + "}\n}\n"
		assertFunctionLengthOutput(t, "cases_test.go", source, "")
	})
}

func TestCaseTableAliasBoundary(t *testing.T) {
	const header = "package example\nfunc example() {\n"
	literal := "[]int{\n" + strings.Repeat("0,\n", 71) + "}\n"
	body := "cases := " + literal + "alias := cases\nfor _, value := range alias[:] {\n_ = value\n}\n"
	t.Run("70 counted lines", func(t *testing.T) {
		assertFunctionLengthOutput(t, "cases_test.go", header+body+strings.Repeat("_ = 0\n", 64)+"}\n", "")
	})
	t.Run("71 counted lines", func(t *testing.T) {
		const want = "cases_test.go:2:1: funclen: function body is 71 lines; limit is 70\n"
		assertFunctionLengthOutput(t, "cases_test.go", header+body+strings.Repeat("_ = 0\n", 65)+"}\n", want)
	})
}
