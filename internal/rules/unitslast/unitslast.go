// Package unitslast reports declarations with a leading unit or qualifier.
package unitslast

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unitslast",
	Doc:  "report Go declarations with a leading unit or qualifier",
	Run:  run,
}

// run checks each declaration once, regardless of how often it is used.
func run(pass *analysis.Pass) (any, error) {
	for ident := range pass.TypesInfo.Defs {
		word := leadingWord(ident.Name)
		switch word {
		case "min", "max",
			"ns", "us", "ms", "seconds", "minutes", "hours",
			"bytes", "kb", "mb", "gb", "kib", "mib", "gib":
			pass.Report(analysis.Diagnostic{
				Pos:      ident.Pos(),
				Category: "unitslast",
				Message:  fmt.Sprintf("move %s to the end of %s", word, ident.Name),
			})
		}
	}
	return nil, nil
}

// leadingWord returns the first word of a compound name, ignoring case.
// In MSTimeout, the capital followed by a lowercase letter starts Timeout.
func leadingWord(name string) string {
	word := []rune(strings.Trim(name, "_"))
	for i := 1; i < len(word); i++ {
		if word[i] == '_' {
			return strings.ToLower(string(word[:i]))
		}
		if unicode.IsUpper(word[i]) &&
			(unicode.IsLower(word[i-1]) || (i+1 < len(word) && unicode.IsLower(word[i+1]))) {
			return strings.ToLower(string(word[:i]))
		}
	}
	return ""
}
