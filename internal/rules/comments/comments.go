// Package comments is the first rule: a Go comment may only be one a tool
// reads. Code explains itself and tests carry the why, so a doc comment is no
// exception, and neither is a comment that says why.
//
// The comments a tool reads, and so the ones left alone, are the //go:
// directives (a build constraint is one) and the line directives, //nolint:
// with a linter and a reason after it, a license header, the marker of a
// generated file and the block of a mutation run's annotation. Every other
// comment is one finding at its start, reported by the Analyzer.
package comments

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Name is the rule's name, the Category of each finding and the word that
// follows its position in a line of output.
const Name = "comments"

const (
	reasonUnread   = "no tool reads this comment: let the code explain itself and a test carry the why"
	reasonNolint   = "a //nolint comment must name its linter and give a reason after it"
	mutationOpen   = "// itos-cc mutation:"
	mutationClose  = "// end itos-cc mutation"
	nolintPrefix   = "//nolint"
	generatedStart = "// Code generated "
	generatedEnd   = " DO NOT EDIT."
)

// Analyzer reports each comment no tool reads. It needs no type information.
var Analyzer = &analysis.Analyzer{
	Name: Name,
	Doc:  "report Go comments that no tool reads",
	Run:  run,
}

var nolintLinters = regexp.MustCompile(`^//nolint:[A-Za-z0-9_-]+(?:,[A-Za-z0-9_-]+)*(\s.*)?$`)

func run(pass *analysis.Pass) (any, error) {
	Report(pass.Files, pass.Report)
	return nil, nil
}

// Report gives each finding of the rule in files to report, in the order of
// the files and of each file's comments.
func Report(files []*ast.File, report func(analysis.Diagnostic)) {
	for _, file := range files {
		for _, d := range findings(file) {
			report(d)
		}
	}
}

func findings(file *ast.File) []analysis.Diagnostic {
	var out []analysis.Diagnostic
	for i, group := range file.Comments {
		header := i == 0 && group.End() < file.Package && isLicense(group)
		annotation := false
		for _, c := range group.List {
			if header {
				continue
			}
			if strings.HasPrefix(c.Text, mutationOpen) {
				annotation = true
			}
			if annotation {
				annotation = c.Text != mutationClose
				continue
			}
			if reason := judge(c); reason != "" {
				out = append(out, diagnostic(c.Slash, reason))
			}
		}
	}
	return out
}

func diagnostic(pos token.Pos, reason string) analysis.Diagnostic {
	return analysis.Diagnostic{Pos: pos, Category: Name, Message: reason}
}

func judge(c *ast.Comment) string {
	text := c.Text
	switch {
	case strings.HasPrefix(text, "//go:"), strings.HasPrefix(text, "//line "), strings.HasPrefix(text, "/*line "):
		return ""
	case strings.HasPrefix(text, nolintPrefix):
		if hasReason(text) {
			return ""
		}
		return reasonNolint
	case strings.HasPrefix(text, generatedStart) && strings.HasSuffix(text, generatedEnd):
		return ""
	}
	return reasonUnread
}

func hasReason(text string) bool {
	m := nolintLinters.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	reason := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m[1]), "//"))
	return reason != ""
}

func isLicense(group *ast.CommentGroup) bool {
	text := strings.ToLower(group.Text())
	return strings.Contains(text, "license") || strings.Contains(text, "licence") || strings.Contains(text, "copyright")
}
