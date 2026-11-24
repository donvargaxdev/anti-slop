// Package comments is the first rule: a Go comment may only be one a tool
// reads. Code explains itself and tests carry the why, so a doc comment is no
// exception, and neither is a comment that says why.
//
// The comments a tool reads, and so the ones left alone, are the //go:
// directives (a build constraint is one) and the line directives, //nolint:
// with a linter and a reason after it, the marker of a generated file, a
// mutation annotation, cgo preambles and cgo export directives. License headers and source
// attribution notices are also preserved. The Analyzer reports every other
// comment at its start.
package comments

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"regexp"
	"slices"
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

const sourceProject = `(?:\bitos(?:-cc|-container|-template)?\b|(?:[\w.-]+/)+[\w.-]+)`

var sourceAttribution = regexp.MustCompile(`(?i)\b(?:(?:adapted|copied|derived|harvested|ported|taken)\s+` +
	`(?:with\s+the\s+owner's\s+permission\s+(?:\([^)]*\)\s+)?)?from|based\s+on)\s+` +
	`(?:the\s+owner's\s+)?(?:https?://)?` + sourceProject +
	`|\bfollows?\s+` + sourceProject + `'s\s+[\w-]+\s+design\b`)

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
	header := licenseHeader(file)
	cgo := cgoComments(file)
	var out []analysis.Diagnostic
	for _, group := range file.Comments {
		if group == header || sourceAttribution.MatchString(group.Text()) {
			continue
		}
		annotation := false
		for _, c := range group.List {
			if cgo[c] {
				continue
			}
			if group.End() < file.Package && constraint.IsPlusBuild(c.Text) {
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

func licenseHeader(file *ast.File) *ast.CommentGroup {
	for _, group := range file.Comments {
		if group.End() >= file.Package {
			break
		}
		if !slices.ContainsFunc(group.List, func(c *ast.Comment) bool {
			return !constraint.IsGoBuild(c.Text) && !constraint.IsPlusBuild(c.Text) && !isGeneratedMarker(c.Text)
		}) {
			continue
		}
		if isLicense(group) {
			return group
		}
		break
	}
	return nil
}

func cgoComments(file *ast.File) map[*ast.Comment]bool {
	comments := make(map[*ast.Comment]bool)
	importsC := false
	for _, decl := range file.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range group.Specs {
			imp, ok := spec.(*ast.ImportSpec)
			if !ok || imp.Path.Value != `"C"` {
				continue
			}
			importsC = true
			doc := imp.Doc
			if doc == nil && len(group.Specs) == 1 {
				doc = group.Doc
			}
			if doc != nil {
				for _, comment := range doc.List {
					comments[comment] = true
				}
			}
		}
	}
	if importsC {
		for _, decl := range file.Decls {
			if directive := cgoExport(decl); directive != nil {
				comments[directive] = true
			}
		}
	}
	return comments
}

func cgoExport(decl ast.Decl) *ast.Comment {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Doc == nil {
		return nil
	}
	for _, c := range fn.Doc.List {
		if name, export := strings.CutPrefix(c.Text, "//export "); export {
			if strings.TrimSpace(name) == fn.Name.Name {
				return c
			}
			break
		}
	}
	return nil
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
	case isGeneratedMarker(text):
		return ""
	}
	return reasonUnread
}

func isGeneratedMarker(text string) bool {
	return strings.HasPrefix(text, generatedStart) && strings.HasSuffix(text, generatedEnd)
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
