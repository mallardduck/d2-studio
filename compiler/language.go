package compiler

import (
	"slices"
	"strings"

	"github.com/d2lang/d2/d2ast"
	"github.com/d2lang/d2/d2lsp"
	"github.com/d2lang/d2/d2target"
)

// LanguageInfo lists D2's vocabulary, taken from the pinned D2 library so
// syntax highlighting never drifts from the compiler.
type LanguageInfo struct {
	// Keywords are all reserved keywords (shape, style, label, ...).
	Keywords []string `json:"keywords"`
	// Values are words that are meaningful as values: shapes, arrowheads,
	// fill patterns, text transforms, near/label positions, directions.
	Values []string `json:"values"`
}

// Language returns D2's keyword and value vocabulary.
func Language() LanguageInfo {
	keywords := make([]string, 0, len(d2ast.ReservedKeywords))
	for k := range d2ast.ReservedKeywords {
		keywords = append(keywords, k)
	}
	slices.Sort(keywords)

	seen := map[string]bool{}
	var values []string
	add := func(vs ...string) {
		for _, v := range vs {
			if v != "" && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
	}
	add(d2target.Shapes...)
	for a := range d2target.Arrowheads {
		add(a)
	}
	add(d2ast.FillPatterns...)
	add(d2ast.TextTransforms...)
	add(d2ast.NearConstantsArray...)
	add(d2ast.LabelPositionsArray...)
	// The values the compiler accepts for `direction`.
	add("up", "down", "right", "left")
	slices.Sort(values)

	return LanguageInfo{Keywords: keywords, Values: values}
}

// Completion is one suggestion for the editor.
type Completion struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	// Insert is the text to insert in place of the word being typed.
	Insert string `json:"insert"`
}

// Complete returns context-aware completions for a cursor position, using
// D2's own completion engine (shape and style names, booleans, fill patterns,
// arrowheads, ...). line and column are zero-based; column counts UTF-16 code
// units so it matches JavaScript and CodeMirror offsets.
//
// Hints that have nothing to insert (for example "(number between 0 and 15)")
// are dropped so accepting a suggestion can never erase what was typed.
func Complete(source string, line, column int) []Completion {
	items, err := d2lsp.GetCompletionItemsUTF16(source, line, column)
	if err != nil {
		return []Completion{}
	}
	out := make([]Completion, 0, len(items))
	for _, it := range items {
		insert := it.InsertText
		if strings.TrimSpace(insert) == "" {
			continue
		}
		out = append(out, Completion{
			Label:  it.Label,
			Kind:   kindName(it.Kind),
			Detail: it.Detail,
			Insert: insert,
		})
	}
	return out
}

func kindName(k d2lsp.CompletionKind) string {
	switch k {
	case d2lsp.StyleCompletion:
		return "style"
	case d2lsp.ShapeCompletion:
		return "shape"
	default:
		return "keyword"
	}
}
