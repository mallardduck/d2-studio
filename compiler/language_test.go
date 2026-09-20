package compiler

import (
	"slices"
	"testing"

	"github.com/d2lang/d2/d2lsp"
)

func TestLanguageComesFromD2(t *testing.T) {
	l := Language()
	for _, k := range []string{"shape", "style", "label", "direction", "near", "classes", "vars", "layers"} {
		if !slices.Contains(l.Keywords, k) {
			t.Errorf("keyword %q missing", k)
		}
	}
	for _, v := range []string{"cylinder", "sql_table", "triangle", "dots", "uppercase", "top-left", "up"} {
		if !slices.Contains(l.Values, v) {
			t.Errorf("value %q missing", v)
		}
	}
	if !slices.IsSorted(l.Keywords) || !slices.IsSorted(l.Values) {
		t.Error("lists should be sorted for stable output")
	}
	seen := map[string]bool{}
	for _, v := range l.Values {
		if seen[v] {
			t.Errorf("duplicate value %q", v)
		}
		seen[v] = true
	}
}

func labels(cs []Completion) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Label
	}
	return out
}

func TestCompleteShapeValues(t *testing.T) {
	// Cursor after "shape: " on zero-based line 1.
	got := Complete("x: {\n  shape: \n}", 1, 9)
	if !slices.Contains(labels(got), "cylinder") {
		t.Fatalf("shape completions = %v", labels(got))
	}
	if got[0].Kind != "shape" || got[0].Insert == "" {
		t.Fatalf("first item = %+v", got[0])
	}
}

func TestCompleteStyleProperties(t *testing.T) {
	got := Complete("x: {\n  style.\n}", 1, 8)
	if !slices.Contains(labels(got), "fill-pattern") {
		t.Fatalf("style completions = %v", labels(got))
	}
}

func TestCompleteDropsHintsWithoutInsertText(t *testing.T) {
	// D2 offers a non-insertable hint after "opacity: ".
	src := "x: {\n  style.opacity: \n}"
	// Guard the premise: D2 really does emit a hint with empty insert text.
	raw, err := d2lsp.GetCompletionItemsUTF16(src, 1, 17)
	if err != nil || len(raw) == 0 || raw[0].InsertText != "" {
		t.Fatalf("premise changed: D2 returned %+v (err %v); update this test", raw, err)
	}
	got := Complete(src, 1, 17)
	for _, c := range got {
		if c.Insert == "" {
			t.Fatalf("hint with empty insert leaked through: %+v", c)
		}
	}
}

func TestCompleteOutOfRangeIsEmptyNotNil(t *testing.T) {
	got := Complete("a -> b", 99, 99)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice (serializes as [] not null)", got)
	}
}

// Columns are UTF-16 units, so text before the cursor containing a
// non-BMP character must not shift the position.
func TestCompleteUsesUTF16Columns(t *testing.T) {
	src := "😀: {\n  shape: \n}"
	got := Complete(src, 1, 9)
	if !slices.Contains(labels(got), "cylinder") {
		t.Fatalf("completions = %v", labels(got))
	}
}
